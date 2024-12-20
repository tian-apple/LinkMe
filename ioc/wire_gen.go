// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package ioc

import (
	"github.com/GoSimplicity/LinkMe/internal/api"
	cache2 "github.com/GoSimplicity/LinkMe/internal/domain/events/cache"
	"github.com/GoSimplicity/LinkMe/internal/domain/events/check"
	"github.com/GoSimplicity/LinkMe/internal/domain/events/email"
	"github.com/GoSimplicity/LinkMe/internal/domain/events/es"
	"github.com/GoSimplicity/LinkMe/internal/domain/events/post"
	"github.com/GoSimplicity/LinkMe/internal/domain/events/publish"
	"github.com/GoSimplicity/LinkMe/internal/domain/events/sms"
	"github.com/GoSimplicity/LinkMe/internal/domain/events/sync"
	"github.com/GoSimplicity/LinkMe/internal/mock"
	"github.com/GoSimplicity/LinkMe/internal/repository"
	"github.com/GoSimplicity/LinkMe/internal/repository/cache"
	"github.com/GoSimplicity/LinkMe/internal/repository/dao"
	"github.com/GoSimplicity/LinkMe/internal/service"
	"github.com/GoSimplicity/LinkMe/pkg/cachep/bloom"
	"github.com/GoSimplicity/LinkMe/pkg/cachep/local"
	"github.com/GoSimplicity/LinkMe/utils/jwt"
)

import (
	_ "github.com/google/wire"
)

// Injectors from wire.go:

func InitWebServer() *Cmd {
	db := InitDB()
	node := InitializeSnowflakeNode()
	logger := InitLogger()
	enforcer := InitCasbin(db)
	userDAO := dao.NewUserDAO(db, node, logger, enforcer)
	cmdable := InitRedis()
	userCache := cache.NewUserCache(cmdable)
	userRepository := repository.NewUserRepository(userDAO, userCache, logger)
	typedClient := InitES()
	searchDAO := dao.NewSearchDAO(db, typedClient, logger)
	searchRepository := repository.NewSearchRepository(searchDAO)
	userService := service.NewUserService(userRepository, logger, searchRepository)
	handler := jwt.NewJWTHandler(cmdable)
	client := InitSaramaClient()
	syncProducer := InitSyncProducer(client)
	producer := sms.NewSaramaSyncProducer(syncProducer, logger)
	emailProducer := email.NewSaramaSyncProducer(syncProducer, logger)
	userHandler := api.NewUserHandler(userService, handler, producer, emailProducer, enforcer)
	mongoClient := InitMongoDB()
	postDAO := dao.NewPostDAO(db, logger, mongoClient)
	cacheBloom := bloom.NewCacheBloom(cmdable)
	cacheManager := local.NewLocalCacheManager(cmdable)
	postRepository := repository.NewPostRepository(postDAO, logger, cacheBloom, cacheManager)
	postProducer := post.NewSaramaSyncProducer(syncProducer)
	publishProducer := publish.NewSaramaSyncProducer(syncProducer, logger)
	postService := service.NewPostService(postRepository, logger, postProducer, publishProducer)
	interactiveDAO := dao.NewInteractiveDAO(db, logger)
	interactiveCache := cache.NewInteractiveCache(cmdable)
	interactiveRepository := repository.NewInteractiveRepository(interactiveDAO, logger, interactiveCache)
	interactiveService := service.NewInteractiveService(interactiveRepository, logger)
	postHandler := api.NewPostHandler(postService, interactiveService, enforcer)
	historyCache := cache.NewHistoryCache(logger, cmdable)
	historyRepository := repository.NewHistoryRepository(logger, historyCache)
	historyService := service.NewHistoryService(historyRepository, logger)
	historyHandler := api.NewHistoryHandler(historyService)
	checkDAO := dao.NewCheckDAO(db, logger)
	checkCache := cache.NewCheckCache(cmdable)
	checkRepository := repository.NewCheckRepository(checkDAO, checkCache, logger)
	activityDAO := dao.NewActivityDAO(db, logger)
	activityRepository := repository.NewActivityRepository(activityDAO)
	checkService := service.NewCheckService(checkRepository, searchRepository, logger, activityRepository)
	checkHandler := api.NewCheckHandler(checkService, enforcer)
	v := InitMiddlewares(handler, logger)
	permissionDAO := dao.NewPermissionDAO(enforcer, logger, db)
	permissionRepository := repository.NewPermissionRepository(logger, permissionDAO)
	permissionService := service.NewPermissionService(permissionRepository, logger)
	permissionHandler := api.NewPermissionHandler(permissionService, enforcer)
	rankingRedisCache := cache.NewRankingRedisCache(cmdable)
	rankingLocalCache := cache.NewRankingLocalCache()
	rankingRepository := repository.NewRankingCache(rankingRedisCache, rankingLocalCache, logger)
	rankingService := service.NewRankingService(interactiveService, postRepository, rankingRepository, logger)
	rankingHandler := api.NewRakingHandler(rankingService)
	plateDAO := dao.NewPlateDAO(logger, db)
	plateRepository := repository.NewPlateRepository(logger, plateDAO)
	plateService := service.NewPlateService(logger, plateRepository)
	plateHandler := api.NewPlateHandler(plateService, enforcer)
	activityService := service.NewActivityService(activityRepository)
	activityHandler := api.NewActivityHandler(activityService, enforcer)
	commentDAO := dao.NewCommentService(db, logger)
	commentRepository := repository.NewCommentRepository(commentDAO)
	commentService := service.NewCommentService(commentRepository)
	commentHandler := api.NewCommentHandler(commentService)
	searchService := service.NewSearchService(searchRepository)
	searchHandler := api.NewSearchHandler(searchService)
	relationDAO := dao.NewRelationDAO(db, logger)
	relationCache := cache.NewRelationCache(cmdable)
	relationRepository := repository.NewRelationRepository(relationDAO, relationCache, logger)
	relationService := service.NewRelationService(relationRepository)
	relationHandler := api.NewRelationHandler(relationService)
	lotteryDrawDAO := dao.NewLotteryDrawDAO(db, logger)
	lotteryDrawRepository := repository.NewLotteryDrawRepository(lotteryDrawDAO, logger)
	lotteryDrawService := service.NewLotteryDrawService(lotteryDrawRepository, logger)
	lotteryDrawHandler := api.NewLotteryDrawHandler(lotteryDrawService)
	engine := InitWeb(userHandler, postHandler, historyHandler, checkHandler, v, permissionHandler, rankingHandler, plateHandler, activityHandler, commentHandler, searchHandler, relationHandler, lotteryDrawHandler)
	cron := InitRanking(logger, rankingService)
	readEventConsumer := post.NewReadEventConsumer(interactiveRepository, client, logger, historyRepository)
	smsDAO := dao.NewSmsDAO(db, logger)
	smsCache := cache.NewSMSCache(cmdable)
	tencentSms := InitSms()
	smsRepository := repository.NewSmsRepository(smsDAO, smsCache, logger, tencentSms)
	smsConsumer := sms.NewSMSConsumer(smsRepository, client, logger, smsCache)
	emailCache := cache.NewEmailCache(cmdable)
	emailRepository := repository.NewEmailRepository(emailCache, logger)
	emailConsumer := email.NewEmailConsumer(emailRepository, client, logger)
	syncConsumer := sync.NewSyncConsumer(client, logger, db, mongoClient, postDAO)
	cacheConsumer := cache2.NewCacheConsumer(client, logger, cmdable, cacheManager, historyCache)
	publishPostEventConsumer := publish.NewPublishPostEventConsumer(checkRepository, client, logger)
	checkConsumer := check.NewCheckConsumer(client, logger, postRepository, checkCache)
	esConsumer := es.NewEsConsumer(client, logger, searchRepository, typedClient)
	v2 := InitConsumers(readEventConsumer, smsConsumer, emailConsumer, syncConsumer, cacheConsumer, publishPostEventConsumer, checkConsumer, esConsumer)
	mockUserRepository := mock.NewMockUserRepository(db, logger, enforcer)
	cmd := &Cmd{
		Server:   engine,
		Cron:     cron,
		Consumer: v2,
		Mock:     mockUserRepository,
	}
	return cmd
}
