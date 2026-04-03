# 🚀 QR Kantin Backend

Bu proje, modern eğitim kurumları ve işletmeler için tasarlanmış, **Go (Golang)** ile geliştirilmiş yüksek performanslı bir kantin yönetim sistemi backend servisidir. Öğrencilerin mobil uygulama üzerinden sipariş vermesini, kantin görevlilerinin ise gerçek zamanlı bir panel üzerinden stok ve sipariş takibi yapmasını sağlar.

## ✨ Öne Çıkan Özellikler

- **⚡ Gerçek Zamanlı İletişim:** WebSocket (Hub mimarisi) kullanılarak yeni siparişlerin kantin paneline anlık düşmesi sağlanır.
- **🍔 Akıllı Stok ve Reçete Yönetimi:** Ürünler reçete tabanlıdır. Bir sipariş onaylandığında, ürünü oluşturan malzemeler stoktan otomatik olarak düşer.
- **💳 Dijital Cüzdan Sistemi:** Kullanıcılar bakiye yükleyebilir ve harcamalarını işlem geçmişi (transaction log) üzerinden takip edebilir.
- **🔒 Güvenlik Katmanları:**
  - **JWT (JSON Web Token):** Tüm API uç noktaları JWT ile korunur.
  - **Bcrypt:** Kullanıcı şifreleri veritabanında güvenli bir şekilde hashlenerek saklanır.
  - **Rol Tabanlı Yetkilendirme:** Admin (Kantinci) ve Öğrenci/Öğretmen rolleri için özel middleware katmanları mevcuttur.
- **📊 Gelişmiş Raporlama:** Zaman aralığına göre kategori, saatlik yoğunluk ve ürün bazlı detaylı satış analizleri sunar.
- **📱 QR Kod ile Teslimat:** Siparişler, güvenliği sağlamak amacıyla benzersiz QR token'lar üzerinden doğrulanarak teslim edilir.

## 🛠 Teknik Yığın (Tech Stack)

- **Dil:** Go (Golang) 1.25.1
- **Framework:** Echo v4 (Web Framework)
- **Veritabanı:** PostgreSQL (SQLx & pgx driver)
- **Real-time:** Gorilla WebSocket
- **Auth:** JWT-v5
- **Validation:** Go-Playground Validator

## 📂 Proje Yapısı (Clean Architecture)

````text
├── cmd
│   └── main.go
├── go.mod
├── go.sum
├── internal
│   ├── db
│   │   └── db.go
│   ├── handlers
│   │   ├── auth.go
│   │   ├── material.go
│   │   ├── order.go
│   │   ├── product.go
│   │   ├── report.go
│   │   ├── user.go
│   │   └── wallet.go
│   ├── middleware
│   │   └── auth_middleware.go
│   ├── models
│   │   ├── base.go
│   │   ├── material.go
│   │   ├── order.go
│   │   ├── product.go
│   │   └── user.go
│   └── websocket
│       ├── client.go
│       └── hub.go
├── LICENSE
└── README.md

## 🚀 Başlangıç

### Gereksinimler
* **Go:** v1.25+
* **PostgreSQL:** Veritabanı işlemleri için
* **.env Dosyası:** Proje kök dizininde yapılandırılmalıdır

### Kurulum ve Çalıştırma
1.  **Projeyi Klonlayın:**
    ```bash
    git clone <repo-url>
    cd qr-kantin-backend
    ```
2.  **Bağımlılıkları Yükleyin:**
    ```bash
    go mod tidy
    ```
3.  **Veritabanı ve Ortam Değişkenlerini Ayarlayın:**
    `.env` dosyanızı şu şekilde düzenleyin:
    ```env
    DB_URL=postgres://kullanici:sifre@localhost:5432/qr_kantin
    JWT_SECRET=senin_cok_gizli_anahtarin
    PORT=1323
    ```
4.  **Uygulamayı Başlatın:**
    ```bash
    go run cmd/main.go
    ```

## 🔌 API Uç Noktaları (Endpoints)

### 🔓 Kamu (Public) Rotaları
* `POST /auth/register` - Öğrenci kayıt başvurusu (Onaysız başlar)
* `POST /auth/login` - Giriş yap ve JWT token al

### 📱 Kullanıcı (API) Rotaları (JWT Gerekli)
* `GET /api/menu` - Aktif ve stokta olan ürün listesi
* `POST /api/order` - Yeni sipariş oluşturma
* `GET /api/wallet/balance` - Güncel bakiye sorgulama
* `GET /api/wallet/history` - İşlem (yükleme/harcama) geçmişi

### 🛠 Yönetici (Admin) Rotaları (Admin Yetkisi Gerekli)
* **Sipariş Yönetimi:**
    * `GET /admin/orders` - Aktif siparişleri görüntüle
    * `PUT /admin/orders/:id/approve` - Siparişi onayla ve stoktan düş
    * `PUT /admin/orders/:id/ready` - Siparişi "Hazır" olarak işaretle
    * `POST /admin/orders/complete` - QR kod ile siparişi teslim et
* **Ürün ve Stok:**
    * `POST /admin/products` - Yeni ürün ve reçete ekle
    * `POST /admin/materials` - Stok girişi yap
* **Kullanıcı ve Rapor:**
    * `GET /admin/users/pending` - Onay bekleyen öğrencileri listele
    * `GET /admin/reports/sales` - Detaylı satış ve ciro analizleri

## ⚖️ Kullanım Şartları ve Lisans

Bu yazılımın tüm telif hakları **Onur Zaim**'e aittir.

1.  **Kişisel ve Eğitim:** Bireysel inceleme ve eğitim amaçlı kullanım serbesttir.
2.  **Ticari Kullanım Yasağı:** Bu yazılımın tamamı veya bir kısmı, yazılı izin alınmaksızın herhangi bir ticari faaliyette, satışta veya gelir getiren bir projede kullanılamaz.
3.  **İletişim:** Ticari kullanım izinleri ve iş birliği için `zaimonur08@gmail.com` adresi üzerinden iletişime geçilmelidir.

---
*© 2026 Onur Zaim - Tüm Hakları Saklıdır.*
````
