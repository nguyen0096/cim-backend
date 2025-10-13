package data

import (
	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/database"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Suppliers contains all test supplier data
func Suppliers() []models.Supplier {
	now := time.Now()

	return []models.Supplier{
		// Tech & Office Suppliers
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Tech Electronics Inc",
			ContactEmail: "contact@techelectronics.com",
			ContactPhone: "+1-555-0123",
			Address:      "123 Silicon Valley Blvd, San Jose, CA 95110",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Office Supply Co",
			ContactEmail: "sales@officesupply.com",
			ContactPhone: "+1-555-0456",
			Address:      "456 Business Park Dr, Dallas, TX 75201",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Global Parts Ltd",
			ContactEmail: "orders@globalparts.com",
			ContactPhone: "+1-555-0789",
			Address:      "789 Industrial Way, Seattle, WA 98101",
		},
		// F&B Suppliers (Vietnam)
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Nông Sản Sạch Việt Nam",
			ContactEmail: "contact@nongsansach.vn",
			ContactPhone: "+84-28-3821-5001",
			Address:      "123 Đường Nguyễn Văn Linh, Quận 7, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Vinamilk - Công ty Sữa Việt Nam",
			ContactEmail: "sales@vinamilk.com.vn",
			ContactPhone: "+84-28-5413-8888",
			Address:      "10 Đường Tân Trào, Phường Tân Phú, Quận 7, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Thủy Sản Miền Trung",
			ContactEmail: "info@thuysanmientrung.vn",
			ContactPhone: "+84-236-3827-100",
			Address:      "45 Đường Nguyễn Văn Linh, TP. Đà Nẵng",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Trung Nguyên Coffee",
			ContactEmail: "order@trungnguyen.com.vn",
			ContactPhone: "+84-500-6789",
			Address:      "12 Đường Thảo Điền, Quận 2, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Gạo Thiên Long",
			ContactEmail: "sales@gaothienlong.vn",
			ContactPhone: "+84-292-3821-456",
			Address:      "234 Quốc Lộ 1A, TP. Cần Thơ",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Gia Vị Việt",
			ContactEmail: "contact@giaviviet.com",
			ContactPhone: "+84-28-3920-7777",
			Address:      "678 Đường Lê Văn Việt, Quận 9, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Vissan - Công ty Thực Phẩm Sài Gòn",
			ContactEmail: "orders@vissan.com.vn",
			ContactPhone: "+84-28-3812-5555",
			Address:      "520 Đường Cách Mạng Tháng Tám, Quận 3, TP.HCM",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Rau Quả Đà Lạt",
			ContactEmail: "sales@rauquadalat.vn",
			ContactPhone: "+84-263-3821-999",
			Address:      "89 Đường Trần Hưng Đạo, TP. Đà Lạt",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Nước Mắm Nam Ngư",
			ContactEmail: "info@namngu.com.vn",
			ContactPhone: "+84-297-3871-234",
			Address:      "101 Đường Trần Phú, TP. Phú Quốc",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Công ty Bánh Kẹo Kinh Đô",
			ContactEmail: "contact@kinhdo.com.vn",
			ContactPhone: "+84-28-5413-7000",
			Address:      "443 Đường Hoàng Văn Thụ, Quận Tân Bình, TP.HCM",
		},
	}
}

// Products contains all test product data
func Products(supplierIDs []uint) []models.Product {
	now := time.Now()

	return []models.Product{
		// Tech & Office Products
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "MacBook Pro 16-inch M3",
			Description: "Professional laptop with M3 chip, 32GB RAM, 1TB SSD",
			ProductType: "laptop",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "LG UltraGear 27\" 4K Gaming Monitor",
			Description: "27-inch 4K UHD gaming monitor with 144Hz refresh rate",
			ProductType: "monitor",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Keychron K8 Mechanical Keyboard",
			Description: "Wireless mechanical keyboard with RGB backlight and hot-swappable switches",
			ProductType: "keyboard",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Logitech MX Master 3S",
			Description: "Advanced wireless mouse with precision scrolling and customizable buttons",
			ProductType: "mouse",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Herman Miller Aeron Chair",
			Description: "Ergonomic office chair with lumbar support and breathable mesh",
			ProductType: "chair",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "UPLIFT Standing Desk 60x30",
			Description: "Height-adjustable standing desk with bamboo top and memory settings",
			ProductType: "desk",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "CalDigit TS4 Thunderbolt 4 Hub",
			Description: "18-port Thunderbolt 4 hub with 98W charging and 40Gbps data transfer",
			ProductType: "hub",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Logitech Brio 4K Webcam",
			Description: "Ultra HD 4K webcam with HDR and Windows Hello support",
			ProductType: "webcam",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sony WH-1000XM5 Headphones",
			Description: "Industry-leading noise cancelling wireless headphones with 30-hour battery",
			ProductType: "headphones",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "iPad Pro 12.9\" M2",
			Description: "12.9-inch iPad Pro with M2 chip, 256GB storage, and Liquid Retina XDR display",
			ProductType: "tablet",
			Unit:        "piece",
			Status:      "active",
		},
		// F&B Products (Vietnam)
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Gạo Tám Thơm ST25",
			Description: "Gạo thơm ST25 đặc sản xuất khẩu, loại 1",
			ProductType: "rice",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cà Phê Robusta Đắk Lắk",
			Description: "Hạt cà phê Robusta nguyên chất từ Tây Nguyên",
			ProductType: "coffee",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Mắm Phú Quốc 40 Độ Đạm",
			Description: "Nước mắm truyền thống Phú Quốc, 40 độ đạm đạm protein",
			ProductType: "condiment",
			Unit:        "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Mì Tươi Sài Gòn",
			Description: "Bánh mì que giòn tươi ngon mỗi ngày",
			ProductType: "bakery",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Rau Xà Lách Đà Lạt",
			Description: "Rau xà lách tươi hữu cơ từ Đà Lạt",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Tươi Vinamilk 100%",
			Description: "Sữa tươi thanh trùng không đường",
			ProductType: "dairy",
			Unit:        "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Tôm Càng Xanh Cần Thơ",
			Description: "Tôm càng xanh tươi sống từ đồng bằng sông Cửu Long",
			ProductType: "seafood",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chả Lụa Đặc Biệt",
			Description: "Chả lụa thượng hạng chất lượng cao",
			ProductType: "meat",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Ô Long Đài Loan",
			Description: "Trà ô long cao cấp nhập khẩu từ Đài Loan",
			ProductType: "tea",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Quy Bơ Kinh Đô",
			Description: "Bánh quy bơ thơm ngon đặc biệt",
			ProductType: "snack",
			Unit:        "box",
			Status:      "active",
		},
		// Additional F&B Products
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bún Tươi",
			Description: "Bún tươi dai ngon từ gạo tẻ",
			ProductType: "noodle",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Phở Khô",
			Description: "Bánh phở khô loại 1",
			ProductType: "noodle",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mì Gói Hảo Hảo",
			Description: "Mì ăn liền vị tôm chua cay",
			ProductType: "instant_noodle",
			Unit:        "carton",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dầu Ăn Simply",
			Description: "Dầu ăn cao cấp chai 1L",
			ProductType: "cooking_oil",
			Unit:        "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Tương Chinsu",
			Description: "Nước tương đậm đà hương vị truyền thống",
			ProductType: "condiment",
			Unit:        "liter",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Tương Ớt Cholimex",
			Description: "Tương ớt cay đặc biệt",
			ProductType: "condiment",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đường Trắng Biên Hòa",
			Description: "Đường cát trắng tinh luyện",
			ProductType: "sugar",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Muối I-ốt",
			Description: "Muối tinh I-ốt sạch",
			ProductType: "salt",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Ngọt Aji-ngon",
			Description: "Bột ngọt tăng vị tự nhiên",
			ProductType: "seasoning",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hạt Nêm Knorr",
			Description: "Hạt nêm thịt thăn xương ống heo",
			ProductType: "seasoning",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cá Ngừ Đóng Hộp VisanFoods",
			Description: "Cá ngừ xốt cà chua 170g",
			ProductType: "canned_food",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Đặc Ông Thọ",
			Description: "Sữa đặc có đường truyền thống",
			ProductType: "dairy",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Chua Vinamilk",
			Description: "Sữa chua có đường lốc 4 hộp",
			ProductType: "dairy",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trứng Gà Tươi",
			Description: "Trứng gà sạch các loại",
			ProductType: "egg",
			Unit:        "tray",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thịt Ba Chỉ Heo",
			Description: "Thịt ba chỉ heo tươi VietGAP",
			ProductType: "meat",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thịt Nạc Vai Heo",
			Description: "Thịt nạc vai heo tươi",
			ProductType: "meat",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thịt Gà Ta",
			Description: "Thịt gà ta sạch",
			ProductType: "poultry",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cá Basa Phi Lê",
			Description: "Cá basa phi lê đông lạnh",
			ProductType: "seafood",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cá Thu Tươi",
			Description: "Cá thu biển tươi ngon",
			ProductType: "seafood",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mực Ống Đông Lạnh",
			Description: "Mực ống sạch đông lạnh",
			ProductType: "seafood",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Rau Muống",
			Description: "Rau muống tươi",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cải Thảo",
			Description: "Cải thảo Đà Lạt",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cà Chua",
			Description: "Cà chua chín đỏ",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hành Tây",
			Description: "Hành tây tím Đà Lạt",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Tỏi",
			Description: "Tỏi tươi Lý Sơn",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Ớt",
			Description: "Ớt hiểm các loại",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Khoai Tây",
			Description: "Khoai tây Đà Lạt",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Su Su",
			Description: "Su su tươi Đà Lạt",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cà Rốt",
			Description: "Cà rốt Đà Lạt",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bí Đỏ",
			Description: "Bí đỏ ngọt tự nhiên",
			ProductType: "produce",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chuối Già",
			Description: "Chuối già chín tự nhiên",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cam Sành",
			Description: "Cam sành Hà Giang",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Xoài Cát Hòa Lộc",
			Description: "Xoài cát Hòa Lộc Tiền Giang",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Thanh Long Ruột Đỏ",
			Description: "Thanh long ruột đỏ Bình Thuận",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sầu Riêng Monthong",
			Description: "Sầu riêng Monthong chuẩn xuất khẩu",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Măng Cụt",
			Description: "Măng cụt tươi miền Tây",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chôm Chôm",
			Description: "Chôm chôm tươi ngọt",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dưa Hấu Không Hạt",
			Description: "Dưa hấu không hạt ngọt lịm",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bơ Booth",
			Description: "Bơ Booth Đắk Lắk",
			ProductType: "fruit",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dừa Xiêm",
			Description: "Dừa xiêm tươi mát",
			ProductType: "fruit",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bia Sài Gòn Xanh",
			Description: "Bia Sài Gòn xanh lager chai 330ml",
			ProductType: "beverage",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bia Tiger",
			Description: "Bia Tiger lon 330ml",
			ProductType: "beverage",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Suối Lavie",
			Description: "Nước khoáng thiên nhiên Lavie 500ml",
			ProductType: "beverage",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Ngọt Coca Cola",
			Description: "Coca Cola lon 330ml",
			ProductType: "beverage",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Ngọt Pepsi",
			Description: "Pepsi Cola lon 330ml",
			ProductType: "beverage",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Xanh Không Độ",
			Description: "Trà xanh không độ C2 chai 455ml",
			ProductType: "beverage",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Cam Ép Minute Maid",
			Description: "Nước cam ép 100% chai 1L",
			ProductType: "beverage",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sữa Tươi TH True Milk",
			Description: "Sữa tươi tiệt trùng hộp 1L",
			ProductType: "dairy",
			Unit:        "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Yaourt Uống Dutch Lady",
			Description: "Yaourt uống lốc 4 chai",
			ProductType: "dairy",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Mì Sandwich",
			Description: "Bánh mì sandwich cắt lát",
			ProductType: "bakery",
			Unit:        "loaf",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Bông Lan Trứng Muối",
			Description: "Bánh bông lan trứng muối thơm ngon",
			ProductType: "bakery",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Mì Que Việt Nam",
			Description: "Bánh mì que giòn tan",
			ProductType: "bakery",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Croissant Bơ",
			Description: "Bánh croissant bơ thơm ngậy",
			ProductType: "bakery",
			Unit:        "piece",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Pía Tân Huê Viên",
			Description: "Bánh pía đậu xanh sầu riêng",
			ProductType: "bakery",
			Unit:        "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Trung Thu Kinh Đô",
			Description: "Bánh trung thu thập cẩm cao cấp",
			ProductType: "bakery",
			Unit:        "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nem Chua Thanh Hóa",
			Description: "Nem chua thanh hóa truyền thống",
			ProductType: "snack",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Kẹo Dừa Bến Tre",
			Description: "Kẹo dừa thơm ngậy đặc sản",
			ProductType: "snack",
			Unit:        "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mứt Tết Hỗn Hợp",
			Description: "Mứt tết gừng, bí, dừa, cà rốt",
			ProductType: "snack",
			Unit:        "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Snack Oishi Vị Tôm",
			Description: "Snack khoai tây Oishi vị tôm",
			ProductType: "snack",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hạt Điều Rang Muối",
			Description: "Hạt điều rang muối Bình Phước",
			ProductType: "snack",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mực Tẩm Gia Vị",
			Description: "Mực tẩm gia vị cay ngọt",
			ProductType: "snack",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Xúc Xích Đức Việt",
			Description: "Xúc xích heo đặc biệt",
			ProductType: "meat",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Giò Lụa",
			Description: "Giò lụa thượng hạng",
			ProductType: "meat",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Giò Thủ",
			Description: "Giò thủ truyền thống",
			ProductType: "meat",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Chả Quế",
			Description: "Chả quế thơm ngon",
			ProductType: "meat",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Pate Gan Heo",
			Description: "Pate gan heo cao cấp",
			ProductType: "meat",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mật Ong Rừng U Minh",
			Description: "Mật ong nguyên chất rừng U Minh",
			ProductType: "condiment",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Canh Heo Quay",
			Description: "Bột canh heo quay đậm đà",
			ProductType: "seasoning",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hạt Tiêu Phú Quốc",
			Description: "Hạt tiêu đen nguyên hạt Phú Quốc",
			ProductType: "spice",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Nghệ Nguyên Chất",
			Description: "Bột nghệ nguyên chất không tẩm",
			ProductType: "spice",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sả Tươi",
			Description: "Sả tươi thơm mạnh",
			ProductType: "spice",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Gừng Tươi",
			Description: "Gừng tươi cay nồng",
			ProductType: "spice",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Dừa Đóng Hộp",
			Description: "Nước dừa tươi đóng hộp Cocoxim",
			ProductType: "beverage",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nước Mía Đóng Chai",
			Description: "Nước mía tươi đóng chai",
			ProductType: "beverage",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Atiso Đà Lạt",
			Description: "Trà atiso giải nhiệt thanh lọc",
			ProductType: "tea",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Trà Sữa Lipton",
			Description: "Trá sữa lon 250ml",
			ProductType: "beverage",
			Unit:        "can",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mì Chính Ajinomoto",
			Description: "Mì chính tinh khiết 400g",
			ProductType: "seasoning",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Giấm Gạo Nhật Bản",
			Description: "Giấm gạo nấu ăn Nhật Bản",
			ProductType: "condiment",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dầu Hào Lee Kum Kee",
			Description: "Dầu hào đặc biệt Lee Kum Kee",
			ProductType: "condiment",
			Unit:        "bottle",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Chiên Giòn",
			Description: "Bột chiên xù giòn lâu",
			ProductType: "flour",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Mì Đa Dụng",
			Description: "Bột mì đa dụng số 8",
			ProductType: "flour",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Năng",
			Description: "Bột năng tinh khiết",
			ProductType: "flour",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Gạo",
			Description: "Bột gạo làm bánh",
			ProductType: "flour",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bột Nở",
			Description: "Bột nở làm bánh",
			ProductType: "flour",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Men Nở Bánh",
			Description: "Men nở bánh mì instant",
			ProductType: "flour",
			Unit:        "pack",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bơ Thực Vật",
			Description: "Bơ thực vật làm bánh",
			ProductType: "dairy",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Cream Cheese Philadelphia",
			Description: "Phô mai kem làm bánh",
			ProductType: "dairy",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Phô Mai Lát Cheddar",
			Description: "Phô mai lát Cheddar hộp 200g",
			ProductType: "dairy",
			Unit:        "box",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Dừa Nạo Khô",
			Description: "Dừa nạo sợi khô",
			ProductType: "ingredient",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Phộng Rang",
			Description: "Đậu phộng rang giòn",
			ProductType: "snack",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Mè Rang",
			Description: "Mè rang trắng thơm",
			ProductType: "ingredient",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Xanh Hạt",
			Description: "Đậu xanh hạt nguyên vỏ",
			ProductType: "ingredient",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Đen",
			Description: "Đậu đen hạt nguyên chất",
			ProductType: "ingredient",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Đậu Đỏ",
			Description: "Đậu đỏ hạt làm chè",
			ProductType: "ingredient",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Nấm Hương Khô",
			Description: "Nấm hương khô cao cấp",
			ProductType: "ingredient",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Miến Dong",
			Description: "Miến dong miền Bắc",
			ProductType: "noodle",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Hủ Tiếu Khô Nam Vang",
			Description: "Hủ tiếu khô Nam Vang đặc biệt",
			ProductType: "noodle",
			Unit:        "kg",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Bánh Đa Nem",
			Description: "Bánh đa nem cuốn loại 1",
			ProductType: "ingredient",
			Unit:        "pack",
			Status:      "active",
		},
	}
}

// InventoryData represents inventory configuration for a product
type InventoryData struct {
	ProductID    uint
	SupplierID   uint
	UnitPrice    float64
	UnitType     string
	Quantity     int
	ReorderLevel int
	Location     string
}

// Inventory contains all test inventory data
func Inventory(productIDs []uint) []models.Inventory {
	now := time.Now()

	// Create inventory locations
	inventories := []models.Inventory{
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Main Warehouse A",
			Description: "Primary storage facility for electronics and office supplies",
			Location:    "123 Industrial Blvd, San Francisco, CA 94107",
			Status:      models.InventoryStatusActive,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Secondary Warehouse B",
			Description: "Secondary storage facility for bulk items",
			Location:    "456 Storage Way, Oakland, CA 94607",
			Status:      models.InventoryStatusActive,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Distribution Center C",
			Description: "Distribution center for fast-moving items",
			Location:    "789 Logistics Ave, San Jose, CA 95110",
			Status:      models.InventoryStatusActive,
		},
	}

	return inventories
}

// createProductSupplierRelationships creates many-to-many relationships between products and suppliers
func createProductSupplierRelationships(tx *gorm.DB, productIDs, supplierIDs []uint) error {
	// Tech products (0-9) supplied by tech suppliers (0-2)
	// F&B products (10+) supplied by multiple F&B suppliers (3-12)
	type ProductSupplier struct {
		ProductID  uint `gorm:"primaryKey"`
		SupplierID uint `gorm:"primaryKey"`
	}
	var productSuppliers []ProductSupplier

	for i, productID := range productIDs {
		if i < 10 {
			// Tech products: assign to tech suppliers (cycling through 0-2)
			supplierIndex := i % 3
			productSuppliers = append(productSuppliers, ProductSupplier{
				ProductID:  productID,
				SupplierID: supplierIDs[supplierIndex],
			})
		} else {
			// F&B products: assign to multiple F&B suppliers (each product gets 3-5 suppliers)
			// Calculate how many suppliers this product should have (3-5)
			numSuppliers := 3 + (i % 3) // Will give 3, 4, or 5 suppliers per product
			fbSupplierStartIndex := 3   // F&B suppliers start at index 3
			fbSupplierCount := 10       // We have 10 F&B suppliers (indices 3-12)

			for j := 0; j < numSuppliers; j++ {
				// Distribute suppliers across products with some variation
				supplierOffset := ((i-10)*2 + j) % fbSupplierCount
				supplierIndex := fbSupplierStartIndex + supplierOffset
				productSuppliers = append(productSuppliers, ProductSupplier{
					ProductID:  productID,
					SupplierID: supplierIDs[supplierIndex],
				})
			}
		}
	}

	if err := tx.Table("product_suppliers").Create(&productSuppliers).Error; err != nil {
		return fmt.Errorf("failed to create product-supplier relationships: %w", err)
	}

	return nil
}

// SeedDatabase populates the database with mock data
func SeedDatabase() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create context with user email for CreatedBy field
	ctx := pkg.WithUserEmail(context.Background(), "seeder@test.com")

	// Start transaction
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Seed in correct order (respecting foreign key constraints)

	// 1. Suppliers
	suppliers := Suppliers()
	if err := tx.Create(&suppliers).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create suppliers: %w", err)
	}
	// Collect supplier IDs after batch creation
	var supplierIDs []uint
	for _, supplier := range suppliers {
		supplierIDs = append(supplierIDs, supplier.ID)
	}

	// 2. Products
	products := Products(nil)
	if err := tx.Create(&products).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create products: %w", err)
	}
	// Collect product IDs after batch creation
	var productIDs []uint
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}

	// 2.5. Create product-supplier relationships
	if err := createProductSupplierRelationships(tx, productIDs, supplierIDs); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create product-supplier relationships: %w", err)
	}

	// 3. Inventories
	inventories := Inventory(productIDs)
	if err := tx.Create(&inventories).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create inventories: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Seed users after database schema is populated
	if err := seedUsers(db); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	return nil
}

// seedUsers populates the database with default users
func seedUsers(db *gorm.DB) error {
	// Initialize Casbin service
	casbinService, err := auth.NewCasbinService(db)
	if err != nil {
		return fmt.Errorf("failed to initialize Casbin service: %w", err)
	}

	// Initialize user repository and service
	userRepo := repository.NewUserRepository(db)
	userService := services.NewUserService(userRepo, casbinService)

	ctx := context.Background()

	// Define default users
	defaultUsers := []struct {
		UID   string
		Email string
		Name  string
		Role  string
	}{
		{
			UID:   "demoAdminUid0000000000000000",
			Email: "test@cim.local",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid000000000000",
			Email: "admin@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid200000000000",
			Email: "admin2@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoAccountantUid00000000000",
			Email: "accountant@cim.local",
			Name:  "Accountant User",
			Role:  string(models.RoleAccountant),
		},
		{
			UID:   "demoStaffUid0000000000000000",
			Email: "staff@cim.local",
			Name:  "Staff User",
			Role:  string(models.RoleStaff),
		},
	}

	// Seed each user
	for _, userData := range defaultUsers {
		// Check if user already exists
		existingUser, err := userService.GetUserByUID(ctx, userData.UID)
		if err != nil {
			log.Printf("Error checking existing user %s: %v", userData.Email, err)
			continue
		}

		if existingUser != nil {
			log.Printf("User %s already exists, skipping", userData.Email)
			continue
		}

		// Create user
		user, err := userService.CreateOrUpdateUser(ctx, userData.UID, userData.Email, userData.Name)
		if err != nil {
			log.Printf("Failed to create user %s: %v", userData.Email, err)
			continue
		}

		// Update role if different from default
		if user.Role != userData.Role {
			err = userService.UpdateUserRole(ctx, user.UID, userData.Role, "system")
			if err != nil {
				log.Printf("Failed to update role for user %s: %v", userData.Email, err)
				continue
			}
		}

		log.Printf("Created user: %s with role: %s", userData.Email, userData.Role)
	}

	log.Println("User seeding completed!")
	return nil
}
