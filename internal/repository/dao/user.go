package dao

import (
	"LinkMe/internal/domain"
	"context"
	"errors"
	sf "github.com/bwmarrin/snowflake"
	"github.com/casbin/casbin/v2"
	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

var (
	ErrCodeDuplicateEmailNumber uint16 = 1062
	ErrDuplicateEmail                  = errors.New("duplicate email")
	ErrUserNotFound                    = errors.New("user not found")
)

type UserDAO interface {
	CreateUser(ctx context.Context, u User) error
	FindByID(ctx context.Context, id int64) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByPhone(ctx context.Context, phone string) (User, error)
	UpdatePasswordByEmail(ctx context.Context, email string, newPassword string) error
	DeleteUser(ctx context.Context, email string, uid int64) error
	UpdateProfile(ctx context.Context, profile domain.Profile) error
	GetProfileByUserID(ctx context.Context, userId int64) (domain.Profile, error)
	ListUser(ctx context.Context, pagination domain.Pagination) ([]domain.UserWithProfileAndRule, error)
	GetUserCount(ctx context.Context) (int64, error)
}

type userDAO struct {
	db   *gorm.DB
	node *sf.Node
	l    *zap.Logger
	ce   *casbin.Enforcer
}

// User 用户信息结构体
type User struct {
	ID           int64   `gorm:"primarykey"`                          // 用户ID，主键
	CreateTime   int64   `gorm:"column:created_at;type:bigint"`       // 创建时间，Unix时间戳
	UpdatedTime  int64   `gorm:"column:updated_at;type:bigint"`       // 更新时间，Unix时间戳
	DeletedTime  int64   `gorm:"column:deleted_at;type:bigint;index"` // 删除时间，Unix时间戳，用于软删除
	PasswordHash string  `gorm:"not null"`                            // 密码哈希值，不能为空
	Deleted      bool    `gorm:"column:deleted;default:false"`        // 删除标志，表示该用户是否被删除
	Email        string  `gorm:"type:varchar(100);uniqueIndex"`       // 邮箱地址，唯一
	Phone        *string `gorm:"type:varchar(15);uniqueIndex"`        // 手机号码，唯一
	Profile      Profile `gorm:"foreignKey:UserID;references:ID"`     // 关联的用户资料
}

// Profile 用户资料信息结构体
type Profile struct {
	ID       int64  `gorm:"primaryKey;autoIncrement"`         // 用户资料ID，主键
	UserID   int64  `gorm:"not null;index"`                   // 用户ID，外键
	NickName string `gorm:"size:50"`                          // 昵称，最大长度50
	Avatar   string `gorm:"type:text"`                        // 头像URL
	About    string `gorm:"type:text"`                        // 个人简介
	Birthday string `gorm:"column:birthday;type:varchar(10)"` // 生日
}

func NewUserDAO(db *gorm.DB, node *sf.Node, l *zap.Logger, ce *casbin.Enforcer) UserDAO {
	return &userDAO{
		db:   db,
		node: node,
		l:    l,
		ce:   ce,
	}
}

// 获取当前时间的时间戳
func (ud *userDAO) currentTime() int64 {
	return time.Now().UnixMilli()
}

// CreateUser 创建用户
func (ud *userDAO) CreateUser(ctx context.Context, u User) error {
	u.CreateTime = ud.currentTime()
	u.UpdatedTime = u.CreateTime
	// 使用雪花算法生成id
	u.ID = ud.node.Generate().Int64()
	// 初始化用户资料
	profile := Profile{
		UserID:   u.ID,
		NickName: "",
		Avatar:   "",
		About:    "",
		Birthday: "",
	}
	// 开始事务
	tx := ud.db.WithContext(ctx).Begin()
	// 创建用户
	if err := tx.Create(&u).Error; err != nil {
		tx.Rollback()
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == ErrCodeDuplicateEmailNumber {
			ud.l.Error("duplicate email error", zap.String("email", u.Email), zap.Error(err))
			return ErrDuplicateEmail
		}
		ud.l.Error("failed to create user", zap.Error(err))
		return err
	}
	// 创建用户资料
	if err := tx.Create(&profile).Error; err != nil {
		tx.Rollback()
		ud.l.Error("failed to create profile", zap.Error(err))
		return err
	}
	// 提交事务
	if err := tx.Commit().Error; err != nil {
		ud.l.Error("transaction commit failed", zap.Error(err))
		return err
	}
	return nil
}

// FindByID 根据ID查询用户数据
func (ud *userDAO) FindByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := ud.db.WithContext(ctx).Where("id = ? AND deleted = ?", id, false).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return user, nil
}

// FindByEmail 根据Email查询用户信息
func (ud *userDAO) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := ud.db.WithContext(ctx).Where("email = ? AND deleted = ?", email, false).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return user, nil
}

// FindByPhone 根据phone查询用户信息
func (ud *userDAO) FindByPhone(ctx context.Context, phone string) (User, error) {
	var user User
	err := ud.db.WithContext(ctx).Where("phone = ? AND deleted = ?", phone).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return user, nil
}

func (ud *userDAO) UpdatePasswordByEmail(ctx context.Context, email string, newPassword string) error {
	tx := ud.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		ud.l.Error("failed to begin transaction", zap.Error(tx.Error))
		return tx.Error
	}
	// 更新密码
	if err := tx.Model(&User{}).Where("email = ? AND deleted = ?", email, false).Update("password_hash", newPassword).Error; err != nil {
		ud.l.Error("update password failed", zap.String("email", email), zap.Error(err))
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			ud.l.Error("failed to rollback transaction", zap.Error(rollbackErr))
		}
		return err
	}
	// 提交事务
	if err := tx.Commit().Error; err != nil {
		ud.l.Error("failed to commit transaction", zap.Error(err))
		return err
	}
	ud.l.Info("password updated successfully", zap.String("email", email))
	return nil
}

func (ud *userDAO) DeleteUser(ctx context.Context, email string, uid int64) error {
	tx := ud.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	if err := tx.Model(&User{}).Where("email = ? AND deleted = ? AND id = ?", email, false, uid).Update("deleted", true).Error; err != nil {
		tx.Rollback()
		ud.l.Error("failed to mark user as deleted", zap.String("email", email), zap.Error(err))
		return err
	}
	if err := tx.Commit().Error; err != nil {
		ud.l.Error("failed to commit transaction", zap.String("email", email), zap.Error(err))
		return err
	}
	ud.l.Info("user marked as deleted", zap.String("email", email))
	return nil
}

// UpdateProfile 更新用户资料
func (ud *userDAO) UpdateProfile(ctx context.Context, profile domain.Profile) error {
	// 创建一个更新用的结构体
	updates := domain.Profile{
		NickName: profile.NickName,
		Avatar:   profile.Avatar,
		About:    profile.About,
		Birthday: profile.Birthday,
	}
	// 更新操作
	err := ud.db.WithContext(ctx).Model(&Profile{}).Where("user_id = ?", profile.UserID).Updates(updates).Error
	if err != nil {
		ud.l.Error("failed to update profile", zap.Error(err))
		return err
	}
	return nil
}

func (ud *userDAO) GetProfileByUserID(ctx context.Context, userId int64) (domain.Profile, error) {
	var profile domain.Profile
	if err := ud.db.WithContext(ctx).Where("user_id = ?", userId).First(&profile).Error; err != nil {
		ud.l.Error("failed to get profile by user id", zap.Error(err))
		return domain.Profile{}, err
	}
	return profile, nil
}

func (ud *userDAO) ListUser(ctx context.Context, pagination domain.Pagination) ([]domain.UserWithProfileAndRule, error) {
	var usersWithProfiles []domain.UserWithProfileAndRule
	intSize := int(*pagination.Size)
	intOffset := int(*pagination.Offset)
	// 执行连接查询
	err := ud.db.WithContext(ctx).
		Table("users").
		Select(`users.id, users.password_hash, users.deleted, users.email, users.phone,
                profiles.id as profile_id, profiles.user_id, profiles.nick_name, profiles.avatar, profiles.about, profiles.birthday`).
		Joins("left join profiles on profiles.user_id = users.id").
		Limit(intSize).
		Offset(intOffset).
		Scan(&usersWithProfiles).Error
	if err != nil {
		ud.l.Error("failed to get all users with profiles", zap.Error(err))
		return nil, err
	}
	// 获取每个用户的角色
	for i, user := range usersWithProfiles {
		roleEmails, er := ud.getUserRoleEmails(ctx, user.ID)
		if er != nil {
			ud.l.Error("failed to get role emails for user", zap.Int64("userID", user.ID), zap.Error(err))
			return nil, er
		}
		if len(roleEmails) > 0 {
			usersWithProfiles[i].Role = strings.Join(roleEmails, ",")
		}
	}
	return usersWithProfiles, nil
}

// getUserRoleEmails 获取给定用户ID的角色电子邮件
func (ud *userDAO) getUserRoleEmails(ctx context.Context, userID int64) ([]string, error) {
	userIDStr := strconv.FormatInt(userID, 10)
	roles, err := ud.ce.GetRolesForUser(userIDStr)
	if err != nil {
		return nil, err
	}
	roleEmails := make([]string, 0, len(roles))
	roleIDs := make([]int64, 0, len(roles))
	for _, roleIDStr := range roles {
		roleID, er := strconv.ParseInt(roleIDStr, 10, 64)
		if er != nil {
			return nil, er
		}
		roleIDs = append(roleIDs, roleID)
	}
	var roleUsers []struct {
		Email string
	}
	err = ud.db.WithContext(ctx).
		Table("users").
		Select("email").
		Where("id IN (?)", roleIDs).
		Scan(&roleUsers).Error

	if err != nil {
		return nil, err
	}
	for _, roleUser := range roleUsers {
		roleEmails = append(roleEmails, roleUser.Email)
	}
	return roleEmails, nil
}

func (ud *userDAO) GetUserCount(ctx context.Context) (int64, error) {
	var count int64
	err := ud.db.WithContext(ctx).Model(&User{}).Count(&count).Error
	if err != nil {
		return -1, err
	}
	return count, nil
}
