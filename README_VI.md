# chatgpt-register

> **Hệ thống Quản lý & Đăng ký Tài khoản ChatGPT Tự động Hàng loạt** · Trình duyệt Không giao diện (Headless) · Đăng ký cực tốc 30 giây · Tỷ lệ thành công 100% · Nhân bản tài khoản con 1-Chạm

---

🌐 **Trang tạo ảnh** [vividai.run](https://vividai.run) &nbsp;|&nbsp;
👥 **Nhóm QQ** [1106849765](https://qm.qq.com/q/1106849765) &nbsp;|&nbsp;
🐧 **QQ** 1114639355 &nbsp;|&nbsp;
🛒 **Cửa hàng** [pay.ldxp.cn/shop/chiyi](https://pay.ldxp.cn/shop/chiyi) &nbsp;|&nbsp;
✉️ **Email** [vividairun@gmail.com](mailto:vividairun@gmail.com)

Ngôn ngữ: [English](README.md) | [Tiếng Việt](README_VI.md) | [中文](README_ZH.md)

---

## ✨ Ưu điểm cốt lõi

| 🚀 Đăng ký cực tốc 30s | ✅ Thành công 100% | 🔁 Nhân bản từ Tài khoản gốc |
|:---:|:---:|:---:|
| Tự động hóa trình duyệt Rod + Anti-detection Stealth, 0% can thiệp thủ công | Tự động đọc mã OTP từ API hòm thư, quy trình khép kín | 1 hòm thư đăng ký 1 tài khoản gốc + N tài khoản phụ alias |

| 🌐 Xoay vòng Proxy Pool | 📊 Bảng điều khiển Trực quan | 📦 Triển khai Độc lập (Zero Dependency) |
|:---:|:---:|:---:|
| Tự động xoay luồng Proxy theo từng tài khoản, đa IP đăng ký đồng thời | Giao diện Glassmorphic hiện đại, Bảng số liệu & Nhật ký chạy thời gian thực | Biên dịch thuần Go thành 1 file duy nhất, không cần cài đặt môi trường phức tạp |

---

## 🤖 Đăng ký Không giao diện (Headless) — Điểm sáng kỹ thuật

> Sử dụng **go-rod + rod/stealth** điều khiển lõi Chromium thực tế, mô phỏng thao tác người thật, vượt qua cơ chế chống bot của OpenAI.

### Quy trình Đăng ký (Tự động 100%)

```
Khởi động Trình duyệt (Tùy chỉnh Headless / Có giao diện)
    ↓
Mở trang Đăng ký ChatGPT & Nạp kịch bản Stealth (Bỏ qua phát hiện bot)
    ↓
Tự động điền Email + Mật khẩu ngẫu nhiên
    ↓
Lắng nghe API Hòm thư thời gian thực, tự động lấy mã OTP 6 số & điền vào
    ↓
Hoàn tất đăng ký → Trích xuất accessToken
    ↓
Giải mã thông tin tài khoản từ accessToken (account_id / user_id / plan_type)
    ↓
Xuất file auth.json đầy đủ (chứa access_token / account_id, v.v.)
    ↓
Lưu vào Cơ sở dữ liệu & Cập nhật trạng thái thành "Đã đăng ký"
```

### Các tính năng kỹ thuật chính

| Tính năng | Mô tả |
|-----------|-------|
| **Chống phát hiện Stealth** | Nạp kịch bản rod/stealth để xóa bỏ cờ `navigator.webdriver` |
| **Đọc mã OTP tự động** | Trực tiếp kết nối API hòm thư (Outlook / Gmail / varymail) kiểm tra mỗi 5s |
| **Đồng bộ IP & Trình duyệt** | Tiến trình đăng ký và yêu cầu API sau đó sử dụng chung 1 địa chỉ Proxy |
| **Nhận diện GeoIP tự động** | Tự động kiểm tra vị trí IP Proxy để thiết lập ngôn ngữ & múi giờ phù hợp |
| **Tự động tải Chromium** | Tự động tải xuống phiên bản Chromium phù hợp ở lần chạy đầu tiên |
| **Chế độ Headless** | Cho phép chạy ẩn trên Server / VPS không có màn hình hiển thị |
| **Chụp ảnh màn hình kiểm thử** | Tự động chụp ảnh lại các bước chính để kiểm tra nguyên nhân thất bại |
| **An toàn Đồng thời (Concurrency)** | Chạy đồng thời nhiều nhiệm vụ với môi trường trình duyệt độc lập |

---

## 🖼️ Xem trước Giao diện

| Bảng điều khiển | Quản lý Tài khoản |
|:---:|:---:|
| ![Bảng điều khiển](./screenshots/dashboard.png) | ![Quản lý Tài khoản](./screenshots/accounts.png) |

| Nhật ký thực thi | Quản lý Hòm thư |
|:---:|:---:|
| ![Nhật ký thực thi](./screenshots/accounts-log.png) | ![Quản lý Hòm thư](./screenshots/mailboxes.png) |

| Đọc thư tự động (Lấy mã xác minh) |
|:---:|
| ![Đọc thư tự động](./screenshots/mailboxes-mail.png) |

---

## 🏗️ Kiến trúc Dự án

```
chatgpt-register/
├── main.go                  # Điểm khởi chạy: Định tuyến Gin & Nhúng tệp tĩnh
├── internal/
│   ├── auth/                # Dịch vụ xác thực JWT (Token đơn, tự gia hạn, lưu database)
│   ├── browserboot/         # Quản lý trình duyệt Rod (Tự động tải Chromium)
│   ├── codexreg/            # Lõi đăng ký ChatGPT (Tự động hóa + Stealth)
│   │   ├── browser.go       # Đóng gói instance Trình duyệt
│   │   ├── codex.go         # Quy trình tự động đăng ký
│   │   ├── geoip.go         # Kiểm tra vị trí địa lý IP
│   │   └── codexreg.go      # Đầu vào nhiệm vụ đăng ký
│   ├── db/                  # Khởi tạo cơ sở dữ liệu SQLite (Driver thuần Go, không CGO)
│   ├── emailalias/          # Tạo alias hòm thư (Nhân bản tài khoản con)
│   ├── handlers/            # Lớp xử lý API HTTP (Gin Handlers)
│   ├── mailfetch/           # Bộ đọc hòm thư (Tự động lấy mã OTP)
│   ├── models/              # Mô hình dữ liệu GORM (Admin / Registration / Mailbox / Setting)
│   ├── producer/            # Bộ điều phối sản xuất hàng loạt & nhân bản
│   ├── replenish/           # Dịch vụ tự động bù tài khoản sang image2api
│   └── varymail/            # Tích hợp API vary.email
└── static/                  # Tệp giao diện tĩnh nhúng sẵn
    ├── i18n/                # Bảng ngữ pháp đa ngôn ngữ (zh.js, en.js, vi.js, i18n.js)
    ├── dashboard.html        # Trang Bảng điều khiển
    ├── accounts.html/js      # Trang Quản lý tài khoản
    ├── mailboxes.html/js     # Trang Quản lý hòm thư
    ├── settings.html         # Trang Cài đặt hệ thống
    ├── login.html            # Trang Đăng nhập
    ├── layout.js             # Bố cục chung & Thanh điều hướng
    └── style.css             # CSS Giao diện kính mờ (35KB)
```

**Công nghệ sử dụng:** Go · Gin · GORM · SQLite (Pure Go) · go-rod · rod/stealth · JWT · HTML5/CSS3/JS thuần

---

## 🚀 Khởi động Nhanh

### Cách 1: Chạy trực tiếp (Khuyên dùng)

Tải file chạy tương ứng với hệ điều hành từ mục Releases:

```bash
# Windows
./chatgpt-register.exe

# Linux
./chatgpt-register-linux
```

Mở trình duyệt truy cập đường dẫn: [http://localhost:9000](http://localhost:9000)

### Cách 2: Chạy từ Mã nguồn

```bash
git clone https://github.com/duyplus/chatgpt-register
cd chatgpt-register
go run .
```

### Cách 3: Tự Biên dịch Binary

```bash
# Windows
go build -o chatgpt-register.exe .

# Linux
GOOS=linux go build -o chatgpt-register-linux .
```

### Tùy chỉnh Cổng (Port)

```bash
ADDR=8080 ./chatgpt-register.exe
```

---

## 🔐 Đăng nhập & Bảo mật

- **Tài khoản mặc định:** `admin` / `admin123`
- Vui lòng đổi mật khẩu tại **Cài đặt Hệ thống** ngay trong lần đăng nhập đầu tiên (Độ dài > 6 ký tự).

---

## ⚙️ Hướng dẫn Sử dụng

### Bước 1: Nhập Hòm thư

Vào mục **Quản lý Hòm thư**, hỗ trợ 2 hình thức:
- **Thêm thủ công:** Điền địa chỉ email, mật khẩu và nhà cung cấp.
- **Nhập hàng loạt:** Nhấn nút "Nhập hòm thư hàng loạt", mỗi dòng một định dạng:
  ```
  email|password|refresh_token|client_id
  ```

### Bước 2: Cấu hình Hệ thống

Vào mục **Cài đặt Hệ thống**, điều chỉnh số luồng đồng thời, số lượng nhân bản sub-account, nguồn hòm thư (`Outlook` hoặc `varymail`) và danh sách Proxy.

### Bước 3: Bắt đầu Sản xuất

1. Vào **Bảng điều khiển**, nhấn nút **Sản xuất**.
2. Nhập số lượng tài khoản target cần tạo.
3. Hệ thống sẽ tự động đăng ký tài khoản gốc → nhân bản tài khoản con → tự động bù đắp nếu có lỗi.

---

## ❓ Câu hỏi Thường gặp (FAQ)

**Q: Lần đầu mở trình duyệt bị chậm?**
> A: Lần đầu chạy hệ thống sẽ tự động tải file Chromium (~150MB). Những lần khởi động sau sẽ diễn ra tức thì.

**Q: Không dùng Proxy có đăng ký được không?**
> A: Có thể. Nếu không điền Proxy hệ thống sẽ kết nối trực tiếp. Tuy nhiên nếu chạy hàng loạt số lượng lớn nên dùng Proxy Pool để tránh bị chặn IP.

---

## ⭐ Star History

Nếu bạn thấy dự án này hữu ích, hãy tặng cho repo 1 Star nhé!
