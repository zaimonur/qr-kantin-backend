package db

import (
	"context"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var Instance *sqlx.DB
var RedisClient *redis.Client

func Connect() {
	// .env dosyasını yükle
	err := godotenv.Load()
	if err != nil {
		log.Fatal(".env dosyası yüklenemedi")
	}

	dsn := os.Getenv("DB_URL")

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatalln("DB Bağlantı Hatası:", err)
	}

	// Bağlantı havuzu ayarları
	db.SetMaxOpenConns(100)          // Aynı anda açık olabilecek maksimum bağlantı
	db.SetMaxIdleConns(50)           // Boşta bekletilecek bağlantı sayısı
	db.SetConnMaxLifetime(time.Hour) // Bağlantıların ömrü

	Instance = db
	log.Println("Sunucudaki DB'ye mermi gibi bağlandık!")

	// 2. Redis Bağlantısı
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379" // Varsayılan
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	// Bağlantı testi
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		log.Println("⚠️ Redis bağlantısı kurulamadı, cache devre dışı kalacak:", err)
	} else {
		log.Println("🚀 Redis mermi gibi bağlandı!")
	}

	log.Println("Sunucudaki DB'ye mermi gibi bağlandık!")
}
