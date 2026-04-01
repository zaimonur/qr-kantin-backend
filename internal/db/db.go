package db

import (
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

var Instance *sqlx.DB

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
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	Instance = db
	log.Println("Sunucudaki DB'ye mermi gibi bağlandık!")
}
