package main

import (
	"cart/common"
	"cart/domain/repository"
	service2 "cart/domain/service"
	"cart/handler"
	pb "cart/proto"
	"fmt"
	"github.com/go-micro/plugins/v4/registry/consul"
	ratelimit "github.com/go-micro/plugins/v4/wrapper/ratelimiter/uber"
	opentracing2 "github.com/go-micro/plugins/v4/wrapper/trace/opentracing"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"go-micro.dev/v4/registry"

	"go-micro.dev/v4"
	"go-micro.dev/v4/logger"
)

var (
	service = "cart"
	version = "latest"
)

var QPS = 100

func main() {
	consulConfig, err := common.GetConsulConfig("127.0.0.1", 8500, "/micro/config")
	consulRegistry := consul.NewRegistry(func(options *registry.Options) {
		options.Addrs = []string{
			"127.0.0.1:8500",
		}
	})
	//链路追踪
	t, io, err := common.NewTracer("go.micro.service.cart", "localhost:6831")
	if err != nil {
		log.Error(err)
	}
	defer io.Close()
	opentracing.SetGlobalTracer(t)
	//数据库连接
	mysqlInfo := common.GetMysqlFromConsul(consulConfig, "mysql")
	fmt.Println(mysqlInfo)
	//创建数据库连接
	db, err := gorm.Open("mysql", mysqlInfo.User+":"+mysqlInfo.Pwd+"@/"+mysqlInfo.Database+"?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Error(err)
	}
	defer db.Close()
	//禁止副表
	db.SingularTable(true)

	//第一次初始化
	//err = repository.NewCartRepository(db).InitTable()
	//if err != nil {
	//	log.Error(err)
	//}
	// Create service
	srv := micro.NewService(
		micro.Name(service),
		micro.Version(version),
		micro.Registry(consulRegistry),
		micro.WrapHandler(opentracing2.NewHandlerWrapper(opentracing.GlobalTracer())),
		micro.WrapHandler(ratelimit.NewHandlerWrapper(QPS)))
	srv.Init()

	cartDataService := service2.NewCartDataService(repository.NewCartRepository(db))

	// Register handler
	if err := pb.RegisterCartHandler(srv.Server(), &handler.Cart{CartDataService: cartDataService}); err != nil {
		logger.Fatal(err)
	}
	// Run service
	if err := srv.Run(); err != nil {
		logger.Fatal(err)
	}
}
