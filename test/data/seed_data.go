package data

import (
	"cim-backend/database"
	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Suppliers contains all test supplier data
func Suppliers() []models.Supplier {
	return []models.Supplier{
		// F&B Suppliers (Vietnam)
		{
			Name:         "Công ty Nông Sản Sạch Việt Nam",
			ContactEmail: "contact@nongsansach.vn",
			ContactPhone: "+84-28-3821-5001",
			Address:      "123 Đường Nguyễn Văn Linh, Quận 7, TP.HCM",
		},
		{

			Name:         "Vinamilk - Công ty Sữa Việt Nam",
			ContactEmail: "sales@vinamilk.com.vn",
			ContactPhone: "+84-28-5413-8888",
			Address:      "10 Đường Tân Trào, Phường Tân Phú, Quận 7, TP.HCM",
		},
		{

			Name:         "Công ty Thủy Sản Miền Trung",
			ContactEmail: "info@thuysanmientrung.vn",
			ContactPhone: "+84-236-3827-100",
			Address:      "45 Đường Nguyễn Văn Linh, TP. Đà Nẵng",
		},
		{

			Name:         "Trung Nguyên Coffee",
			ContactEmail: "order@trungnguyen.com.vn",
			ContactPhone: "+84-500-6789",
			Address:      "12 Đường Thảo Điền, Quận 2, TP.HCM",
		},
		{

			Name:         "Công ty Gạo Thiên Long",
			ContactEmail: "sales@gaothienlong.vn",
			ContactPhone: "+84-292-3821-456",
			Address:      "234 Quốc Lộ 1A, TP. Cần Thơ",
		},
		{

			Name:         "Công ty Gia Vị Việt",
			ContactEmail: "contact@giaviviet.com",
			ContactPhone: "+84-28-3920-7777",
			Address:      "678 Đường Lê Văn Việt, Quận 9, TP.HCM",
		},
		{

			Name:         "Vissan - Công ty Thực Phẩm Sài Gòn",
			ContactEmail: "orders@vissan.com.vn",
			ContactPhone: "+84-28-3812-5555",
			Address:      "520 Đường Cách Mạng Tháng Tám, Quận 3, TP.HCM",
		},
		{

			Name:         "Công ty Rau Quả Đà Lạt",
			ContactEmail: "sales@rauquadalat.vn",
			ContactPhone: "+84-263-3821-999",
			Address:      "89 Đường Trần Hưng Đạo, TP. Đà Lạt",
		},
		{

			Name:         "Công ty Nước Mắm Nam Ngư",
			ContactEmail: "info@namngu.com.vn",
			ContactPhone: "+84-297-3871-234",
			Address:      "101 Đường Trần Phú, TP. Phú Quốc",
		},
		{

			Name:         "Công ty Bánh Kẹo Kinh Đô",
			ContactEmail: "contact@kinhdo.com.vn",
			ContactPhone: "+84-28-5413-7000",
			Address:      "443 Đường Hoàng Văn Thụ, Quận Tân Bình, TP.HCM",
		},
	}
}

type ProductSeed struct {
	Base        models.Base
	Name        string
	Description string
	ProductType string
	UnitSymbol  string
	Status      string
}

// Units returns the measurement units used in test data
func Units() []models.Unit {
	return []models.Unit{
		{
			UnitType:         "mass",
			Name:             "KILOGRAM",
			Symbol:           "kg",
			ConversionFactor: 1,
		},
		{

			UnitType:         "volume",
			Name:             "LITER",
			Symbol:           "liter",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "PIECE",
			Symbol:           "piece",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "BOX",
			Symbol:           "box",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "CARTON",
			Symbol:           "carton",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "BOTTLE",
			Symbol:           "bottle",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "CAN",
			Symbol:           "can",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "PACK",
			Symbol:           "pack",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "LOAF",
			Symbol:           "loaf",
			ConversionFactor: 1,
		},
		{

			UnitType:         "count",
			Name:             "TRAY",
			Symbol:           "tray",
			ConversionFactor: 1,
		},
	}
}

// productSeeds contains all test product data
func productSeeds() []ProductSeed {
	return []ProductSeed{
		// F&B Products (Vietnam)
		{

			Name:        "Gạo Tám Thơm ST25",
			Description: "Gạo thơm ST25 đặc sản xuất khẩu, loại 1",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cà Phê Robusta Đắk Lắk",
			Description: "Hạt cà phê Robusta nguyên chất từ Tây Nguyên",
			ProductType: "Nước",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Nước Mắm Phú Quốc 40 Độ Đạm",
			Description: "Nước mắm truyền thống Phú Quốc, 40 độ đạm đạm protein",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{

			Name:        "Bánh Mì Tươi Sài Gòn",
			Description: "Bánh mì que giòn tươi ngon mỗi ngày",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{

			Name:        "Rau Xà Lách Đà Lạt",
			Description: "Rau xà lách tươi hữu cơ từ Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Sữa Tươi Vinamilk 100%",
			Description: "Sữa tươi thanh trùng không đường",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{

			Name:        "Tôm Càng Xanh Cần Thơ",
			Description: "Tôm càng xanh tươi sống từ đồng bằng sông Cửu Long",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Chả Lụa Đặc Biệt",
			Description: "Chả lụa thượng hạng chất lượng cao",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Trà Ô Long Đài Loan",
			Description: "Trà ô long cao cấp nhập khẩu từ Đài Loan",
			ProductType: "Nước",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bánh Quy Bơ Kinh Đô",
			Description: "Bánh quy bơ thơm ngon đặc biệt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		// Additional F&B Products
		{

			Name:        "Bún Tươi",
			Description: "Bún tươi dai ngon từ gạo tẻ",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Phở Khô",
			Description: "Bánh phở khô loại 1",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Mì Gói Hảo Hảo",
			Description: "Mì ăn liền vị tôm chua cay",
			ProductType: "Cơm",
			UnitSymbol:  "carton",
			Status:      "active",
		},
		{

			Name:        "Dầu Ăn Simply",
			Description: "Dầu ăn cao cấp chai 1L",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{

			Name:        "Nước Tương Chinsu",
			Description: "Nước tương đậm đà hương vị truyền thống",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "liter",
			Status:      "active",
		},
		{

			Name:        "Tương Ớt Cholimex",
			Description: "Tương ớt cay đặc biệt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Đường Trắng Biên Hòa",
			Description: "Đường cát trắng tinh luyện",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Muối I-ốt",
			Description: "Muối tinh I-ốt sạch",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bột Ngọt Aji-ngon",
			Description: "Bột ngọt tăng vị tự nhiên",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Hạt Nêm Knorr",
			Description: "Hạt nêm thịt thăn xương ống heo",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cá Ngừ Đóng Hộp VisanFoods",
			Description: "Cá ngừ xốt cà chua 170g",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Sữa Đặc Ông Thọ",
			Description: "Sữa đặc có đường truyền thống",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Sữa Chua Vinamilk",
			Description: "Sữa chua có đường lốc 4 hộp",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Trứng Gà Tươi",
			Description: "Trứng gà sạch các loại",
			ProductType: "Cơm",
			UnitSymbol:  "tray",
			Status:      "active",
		},
		{

			Name:        "Thịt Ba Chỉ Heo",
			Description: "Thịt ba chỉ heo tươi VietGAP",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Thịt Nạc Vai Heo",
			Description: "Thịt nạc vai heo tươi",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Thịt Gà Ta",
			Description: "Thịt gà ta sạch",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cá Basa Phi Lê",
			Description: "Cá basa phi lê đông lạnh",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cá Thu Tươi",
			Description: "Cá thu biển tươi ngon",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Mực Ống Đông Lạnh",
			Description: "Mực ống sạch đông lạnh",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Rau Muống",
			Description: "Rau muống tươi",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cải Thảo",
			Description: "Cải thảo Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cà Chua",
			Description: "Cà chua chín đỏ",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Hành Tây",
			Description: "Hành tây tím Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Tỏi",
			Description: "Tỏi tươi Lý Sơn",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Ớt",
			Description: "Ớt hiểm các loại",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Khoai Tây",
			Description: "Khoai tây Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Su Su",
			Description: "Su su tươi Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cà Rốt",
			Description: "Cà rốt Đà Lạt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bí Đỏ",
			Description: "Bí đỏ ngọt tự nhiên",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Chuối Già",
			Description: "Chuối già chín tự nhiên",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cam Sành",
			Description: "Cam sành Hà Giang",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Xoài Cát Hòa Lộc",
			Description: "Xoài cát Hòa Lộc Tiền Giang",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Thanh Long Ruột Đỏ",
			Description: "Thanh long ruột đỏ Bình Thuận",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Sầu Riêng Monthong",
			Description: "Sầu riêng Monthong chuẩn xuất khẩu",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Măng Cụt",
			Description: "Măng cụt tươi miền Tây",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Chôm Chôm",
			Description: "Chôm chôm tươi ngọt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Dưa Hấu Không Hạt",
			Description: "Dưa hấu không hạt ngọt lịm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bơ Booth",
			Description: "Bơ Booth Đắk Lắk",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Dừa Xiêm",
			Description: "Dừa xiêm tươi mát",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{

			Name:        "Bia Sài Gòn Xanh",
			Description: "Bia Sài Gòn xanh lager chai 330ml",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Bia Tiger",
			Description: "Bia Tiger lon 330ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Nước Suối Lavie",
			Description: "Nước khoáng thiên nhiên Lavie 500ml",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Nước Ngọt Coca Cola",
			Description: "Coca Cola lon 330ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Nước Ngọt Pepsi",
			Description: "Pepsi Cola lon 330ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Trà Xanh Không Độ",
			Description: "Trà xanh không độ C2 chai 455ml",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Nước Cam Ép Minute Maid",
			Description: "Nước cam ép 100% chai 1L",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Sữa Tươi TH True Milk",
			Description: "Sữa tươi tiệt trùng hộp 1L",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{

			Name:        "Yaourt Uống Dutch Lady",
			Description: "Yaourt uống lốc 4 chai",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Bánh Mì Sandwich",
			Description: "Bánh mì sandwich cắt lát",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "loaf",
			Status:      "active",
		},
		{

			Name:        "Bánh Bông Lan Trứng Muối",
			Description: "Bánh bông lan trứng muối thơm ngon",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{

			Name:        "Bánh Mì Que Việt Nam",
			Description: "Bánh mì que giòn tan",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{

			Name:        "Bánh Croissant Bơ",
			Description: "Bánh croissant bơ thơm ngậy",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "piece",
			Status:      "active",
		},
		{

			Name:        "Bánh Pía Tân Huê Viên",
			Description: "Bánh pía đậu xanh sầu riêng",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{

			Name:        "Bánh Trung Thu Kinh Đô",
			Description: "Bánh trung thu thập cẩm cao cấp",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{

			Name:        "Nem Chua Thanh Hóa",
			Description: "Nem chua thanh hóa truyền thống",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Kẹo Dừa Bến Tre",
			Description: "Kẹo dừa thơm ngậy đặc sản",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{

			Name:        "Mứt Tết Hỗn Hợp",
			Description: "Mứt tết gừng, bí, dừa, cà rốt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{

			Name:        "Snack Oishi Vị Tôm",
			Description: "Snack khoai tây Oishi vị tôm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Hạt Điều Rang Muối",
			Description: "Hạt điều rang muối Bình Phước",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Mực Tẩm Gia Vị",
			Description: "Mực tẩm gia vị cay ngọt",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Xúc Xích Đức Việt",
			Description: "Xúc xích heo đặc biệt",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Giò Lụa",
			Description: "Giò lụa thượng hạng",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Giò Thủ",
			Description: "Giò thủ truyền thống",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Chả Quế",
			Description: "Chả quế thơm ngon",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Pate Gan Heo",
			Description: "Pate gan heo cao cấp",
			ProductType: "Cơm",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Mật Ong Rừng U Minh",
			Description: "Mật ong nguyên chất rừng U Minh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Bột Canh Heo Quay",
			Description: "Bột canh heo quay đậm đà",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Hạt Tiêu Phú Quốc",
			Description: "Hạt tiêu đen nguyên hạt Phú Quốc",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bột Nghệ Nguyên Chất",
			Description: "Bột nghệ nguyên chất không tẩm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Sả Tươi",
			Description: "Sả tươi thơm mạnh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Gừng Tươi",
			Description: "Gừng tươi cay nồng",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Nước Dừa Đóng Hộp",
			Description: "Nước dừa tươi đóng hộp Cocoxim",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Nước Mía Đóng Chai",
			Description: "Nước mía tươi đóng chai",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Trà Atiso Đà Lạt",
			Description: "Trà atiso giải nhiệt thanh lọc",
			ProductType: "Nước",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Trà Sữa Lipton",
			Description: "Trá sữa lon 250ml",
			ProductType: "Nước",
			UnitSymbol:  "can",
			Status:      "active",
		},
		{

			Name:        "Mì Chính Ajinomoto",
			Description: "Mì chính tinh khiết 400g",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Giấm Gạo Nhật Bản",
			Description: "Giấm gạo nấu ăn Nhật Bản",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Dầu Hào Lee Kum Kee",
			Description: "Dầu hào đặc biệt Lee Kum Kee",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "bottle",
			Status:      "active",
		},
		{

			Name:        "Bột Chiên Giòn",
			Description: "Bột chiên xù giòn lâu",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bột Mì Đa Dụng",
			Description: "Bột mì đa dụng số 8",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bột Năng",
			Description: "Bột năng tinh khiết",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bột Gạo",
			Description: "Bột gạo làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bột Nở",
			Description: "Bột nở làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Men Nở Bánh",
			Description: "Men nở bánh mì instant",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
		{

			Name:        "Bơ Thực Vật",
			Description: "Bơ thực vật làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Cream Cheese Philadelphia",
			Description: "Phô mai kem làm bánh",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Phô Mai Lát Cheddar",
			Description: "Phô mai lát Cheddar hộp 200g",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "box",
			Status:      "active",
		},
		{

			Name:        "Dừa Nạo Khô",
			Description: "Dừa nạo sợi khô",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Đậu Phộng Rang",
			Description: "Đậu phộng rang giòn",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Mè Rang",
			Description: "Mè rang trắng thơm",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Đậu Xanh Hạt",
			Description: "Đậu xanh hạt nguyên vỏ",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Đậu Đen",
			Description: "Đậu đen hạt nguyên chất",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Đậu Đỏ",
			Description: "Đậu đỏ hạt làm chè",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Nấm Hương Khô",
			Description: "Nấm hương khô cao cấp",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Miến Dong",
			Description: "Miến dong miền Bắc",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Hủ Tiếu Khô Nam Vang",
			Description: "Hủ tiếu khô Nam Vang đặc biệt",
			ProductType: "Cơm",
			UnitSymbol:  "kg",
			Status:      "active",
		},
		{

			Name:        "Bánh Đa Nem",
			Description: "Bánh đa nem cuốn loại 1",
			ProductType: "Ăn nhẹ",
			UnitSymbol:  "pack",
			Status:      "active",
		},
	}
}

// Products converts product seeds into persisted models with resolved unit IDs
func Products(unitIDs map[string]uint) []models.Product {
	seeds := productSeeds()
	products := make([]models.Product, 0, len(seeds))

	for _, seed := range seeds {
		unitID, ok := unitIDs[seed.UnitSymbol]
		if !ok {
			if fallback, hasFallback := unitIDs["piece"]; hasFallback {
				unitID = fallback
			} else {
				continue
			}
		}

		products = append(products, models.Product{
			Base:        seed.Base,
			Name:        seed.Name,
			Description: seed.Description,
			ProductType: seed.ProductType,
			UnitID:      unitID,
			Status:      seed.Status,
		})
	}

	return products
}

// InventoryData represents inventory configuration for a product
type InventoryData struct {
	ProductID    uint
	SupplierID   uint
	UnitPrice    float64
	UnitType     string
	Quantity     float64
	ReorderLevel int
	Location     string
}

// Inventory contains all test inventory data
func Inventory(productIDs []uint) []models.Inventory {

	// Create inventory locations
	inventories := []models.Inventory{
		{

			Name:        "Main Warehouse A",
			Description: "Primary storage facility for electronics and office supplies",
			Location:    "123 Industrial Blvd, San Francisco, CA 94107",
			Status:      models.InventoryStatusActive,
		},
		{

			Name:        "Secondary Warehouse B",
			Description: "Secondary storage facility for bulk items",
			Location:    "456 Storage Way, Oakland, CA 94607",
			Status:      models.InventoryStatusActive,
		},
		{

			Name:        "Distribution Center C",
			Description: "Distribution center for fast-moving items",
			Location:    "789 Logistics Ave, San Jose, CA 95110",
			Status:      models.InventoryStatusActive,
		},
	}

	return inventories
}

// Menus contains all test menu data
func Menus() []models.Menu {

	return []models.Menu{
		{

			Name: "Thực Đơn Chính",
		},
		{

			Name: "Thực Đơn Sáng",
		},
		{

			Name: "Thực Đơn Trưa",
		},
		{

			Name: "Thực Đơn Tối",
		},
		{

			Name: "Thực Đơn Tráng Miệng",
		},
		{

			Name: "Thực Đơn Đồ Uống",
		},
		{

			Name: "Thực Đơn Đặc Biệt",
		},
		{

			Name: "Thực Đơn Buffet",
		},
	}
}

// MenuItems contains all test menu item data
func MenuItems() []models.MenuItem {

	return []models.MenuItem{
		{

			Name: "Phở Bò",
		},
		{

			Name: "Phở Gà",
		},
		{

			Name: "Bún Bò Huế",
		},
		{

			Name: "Bún Chả",
		},
		{

			Name: "Bánh Mì Thịt Nướng",
		},
		{

			Name: "Bánh Mì Pate",
		},
		{

			Name: "Cơm Tấm Sườn Bì Chả",
		},
		{

			Name: "Cơm Gà",
		},
		{

			Name: "Cơm Tôm Rang",
		},
		{

			Name: "Bánh Xèo",
		},
		{

			Name: "Gỏi Cuốn",
		},
		{

			Name: "Chả Giò",
		},
		{

			Name: "Nem Nướng",
		},
		{

			Name: "Bánh Cuốn",
		},
		{

			Name: "Cháo Lòng",
		},
		{

			Name: "Hủ Tiếu Nam Vang",
		},
		{

			Name: "Mì Quảng",
		},
		{

			Name: "Bún Riêu",
		},
		{

			Name: "Bún Mắm",
		},
		{

			Name: "Cà Phê Đen",
		},
		{

			Name: "Cà Phê Sữa",
		},
		{

			Name: "Trà Đá",
		},
		{

			Name: "Nước Mía",
		},
		{

			Name: "Sinh Tố",
		},
		{

			Name: "Chè Đậu Xanh",
		},
		{

			Name: "Chè Ba Màu",
		},
		{

			Name: "Bánh Flan",
		},
		{

			Name: "Kem Dừa",
		},
		{

			Name: "Bánh Mì Chả Cá",
		},
		{

			Name: "Bánh Mì Xíu Mại",
		},
		{

			Name: "Cơm Cháy",
		},
		{

			Name: "Canh Chua Cá",
		},
		{

			Name: "Lẩu Thái",
		},
		{

			Name: "Lẩu Hải Sản",
		},
	}
}

// createMenuItemMenuRelationships creates many-to-many relationships between menu items and menus
func createMenuItemMenuRelationships(tx *gorm.DB, menuItemIDs, menuIDs []uint) error {
	type MenuMenuItem struct {
		MenuItemID uint `gorm:"primaryKey"`
		MenuID     uint `gorm:"primaryKey"`
	}
	var menuMenuItems []MenuMenuItem

	// Distribute menu items across menus logically
	// Breakfast items go to morning menu, drinks to drink menu, etc.
	for i, menuItemID := range menuItemIDs {
		// Each menu item can belong to 1-3 menus
		numMenus := 1 + (i % 3)

		// Map menu items to appropriate menus based on their type
		var assignedMenus []uint
		if i < 5 {
			// Breakfast items (Phở, Bún, Bánh Mì) -> Thực Đơn Sáng, Thực Đơn Chính
			assignedMenus = []uint{menuIDs[1], menuIDs[0]} // Sáng, Chính
		} else if i < 19 {
			// Main dishes -> Thực Đơn Trưa, Thực Đơn Tối, Thực Đơn Chính
			assignedMenus = []uint{menuIDs[2], menuIDs[3], menuIDs[0]} // Trưa, Tối, Chính
		} else if i < 25 {
			// Drinks -> Thực Đơn Đồ Uống, Thực Đơn Chính
			assignedMenus = []uint{menuIDs[5], menuIDs[0]} // Đồ Uống, Chính
		} else if i < 30 {
			// Desserts -> Thực Đơn Tráng Miệng, Thực Đơn Chính
			assignedMenus = []uint{menuIDs[4], menuIDs[0]} // Tráng Miệng, Chính
		} else {
			// Special items -> Thực Đơn Đặc Biệt, Thực Đơn Chính
			assignedMenus = []uint{menuIDs[6], menuIDs[0]} // Đặc Biệt, Chính
		}

		// Assign to menus
		for j := 0; j < numMenus && j < len(assignedMenus); j++ {
			menuMenuItems = append(menuMenuItems, MenuMenuItem{
				MenuItemID: menuItemID,
				MenuID:     assignedMenus[j],
			})
		}
	}

	if err := tx.Table("menu_menu_items").Create(&menuMenuItems).Error; err != nil {
		return fmt.Errorf("failed to create menu-item-menu relationships: %w", err)
	}

	return nil
}

// createMenuItemProductRelationships creates many-to-many relationships between menu items and products
func createMenuItemProductRelationships(tx *gorm.DB, menuItemIDs, productIDs []uint, products []models.Product) error {
	type MenuItemProduct struct {
		MenuItemID uint `gorm:"primaryKey"`
		ProductID  uint `gorm:"primaryKey"`
	}
	var menuItemProducts []MenuItemProduct

	// Map menu items to relevant products based on Vietnamese cuisine
	// Each menu item gets 2-6 products that are typically used in that dish
	for i, menuItemID := range menuItemIDs {
		numProducts := 2 + (i % 5) // 2-6 products per menu item

		// Find relevant products based on menu item name patterns
		var relevantProductIndices []int
		menuItemName := MenuItems()[i].Name

		// Map menu items to product types based on dish characteristics
		if contains(menuItemName, []string{"Phở", "Bún", "Hủ Tiếu", "Mì"}) {
			// Noodle dishes need: noodles, meat, vegetables, spices
			relevantProductIndices = findProductsByType(products, []string{"Cơm", "Ăn nhẹ"})
		} else if contains(menuItemName, []string{"Bánh Mì", "Bánh"}) {
			// Bread items need: bread, meat, vegetables
			relevantProductIndices = findProductsByType(products, []string{"Ăn nhẹ"})
		} else if contains(menuItemName, []string{"Cơm"}) {
			// Rice dishes need: rice, meat, vegetables
			relevantProductIndices = findProductsByType(products, []string{"Cơm", "Ăn nhẹ"})
		} else if contains(menuItemName, []string{"Cà Phê", "Trà", "Nước", "Sinh Tố"}) {
			// Drinks need: beverages, sugar, etc.
			relevantProductIndices = findProductsByType(products, []string{"Nước", "Ăn nhẹ"})
		} else if contains(menuItemName, []string{"Chè", "Kem", "Bánh Flan"}) {
			// Desserts need: sugar, milk, fruits
			relevantProductIndices = findProductsByType(products, []string{"Ăn nhẹ"})
		} else {
			// Default: mix of products
			relevantProductIndices = findProductsByType(products, []string{"Cơm", "Ăn nhẹ"})
		}

		// If no specific products found, use a general selection
		if len(relevantProductIndices) == 0 {
			for j := 0; j < len(productIDs) && j < 20; j++ {
				relevantProductIndices = append(relevantProductIndices, j)
			}
		}

		// Assign products to menu item
		for j := 0; j < numProducts && j < len(relevantProductIndices); j++ {
			productIndex := relevantProductIndices[(i+j)%len(relevantProductIndices)]
			if productIndex < len(productIDs) {
				menuItemProducts = append(menuItemProducts, MenuItemProduct{
					MenuItemID: menuItemID,
					ProductID:  productIDs[productIndex],
				})
			}
		}
	}

	if err := tx.Table("menu_item_products").Create(&menuItemProducts).Error; err != nil {
		return fmt.Errorf("failed to create menu-item-product relationships: %w", err)
	}

	return nil
}

// contains checks if a string contains any of the substrings
func contains(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// findProductsByType finds product indices that match the given product types
func findProductsByType(products []models.Product, productTypes []string) []int {
	var indices []int
	for i, product := range products {
		for _, productType := range productTypes {
			if product.ProductType == productType {
				indices = append(indices, i)
				break
			}
		}
	}
	// If no matches found, return first 20 products as fallback
	if len(indices) == 0 {
		max := 20
		if len(products) < max {
			max = len(products)
		}
		for i := 0; i < max; i++ {
			indices = append(indices, i)
		}
	}
	return indices
}

// createProductSupplierRelationships creates many-to-many relationships between products and suppliers
func createProductSupplierRelationships(tx *gorm.DB, productIDs, supplierIDs []uint) error {
	// F&B products supplied by multiple F&B suppliers (each product gets 3-5 suppliers)
	type ProductSupplier struct {
		ProductID  uint `gorm:"primaryKey"`
		SupplierID uint `gorm:"primaryKey"`
	}
	var productSuppliers []ProductSupplier

	for i, productID := range productIDs {
		// F&B products: assign to multiple F&B suppliers (each product gets 3-5 suppliers)
		// Calculate how many suppliers this product should have (3-5)
		numSuppliers := 3 + (i % 3)         // Will give 3, 4, or 5 suppliers per product
		fbSupplierCount := len(supplierIDs) // All suppliers are now F&B suppliers

		for j := 0; j < numSuppliers; j++ {
			// Distribute suppliers across products with some variation
			supplierOffset := (i*2 + j) % fbSupplierCount
			supplierIndex := supplierOffset
			productSuppliers = append(productSuppliers, ProductSupplier{
				ProductID:  productID,
				SupplierID: supplierIDs[supplierIndex],
			})
		}
	}

	if err := tx.Table("product_suppliers").Create(&productSuppliers).Error; err != nil {
		return fmt.Errorf("failed to create product-supplier relationships: %w", err)
	}

	return nil
}

// SaleOrders contains all test sale order data
func SaleOrders(inventoryIDs []uint) []models.SaleOrder {
	now := time.Now()
	orders := make([]models.SaleOrder, 0, 20)

	// Generate test customer IDs (26 characters each)
	testCustomerIDs := []string{
		"customer001123456789012345",
		"customer002123456789012345",
		"customer003123456789012345",
		"customer004123456789012345",
		"customer005123456789012345",
	}

	// Generate sale orders with various statuses
	statuses := []models.SaleOrderStatus{
		models.SaleOrderStatusOrdered,
		models.SaleOrderStatusServed,
		models.SaleOrderStatusCompleted,
		models.SaleOrderStatusCompleted,
		models.SaleOrderStatusCancelled,
	}

	for i := 0; i < 20; i++ {
		// Use inventory IDs in round-robin fashion
		inventoryID := inventoryIDs[i%len(inventoryIDs)]

		// Generate order number: SO-YYMMDD-HHMMSS-XX
		// Using sequential alphanumeric suffixes for simplicity in seed data
		orderTime := now.Add(-time.Duration(i) * time.Hour)
		// Generate 2-character alphanumeric suffix (A0-Z9)
		suffixChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		suffix := string([]byte{
			suffixChars[(i*2)%len(suffixChars)],
			suffixChars[(i*2+1)%len(suffixChars)],
		})
		orderNumber := fmt.Sprintf("SO-%s-%s",
			orderTime.Format("060102-150405"),
			suffix)

		// Assign status based on index
		status := statuses[i%len(statuses)]

		// Assign customer ID
		customerID := testCustomerIDs[i%len(testCustomerIDs)]

		orders = append(orders, models.SaleOrder{
			Base: models.Base{
				CreatedAt: orderTime,
				UpdatedAt: orderTime,
			},
			CustomerID:  customerID,
			OrderNumber: orderNumber,
			InventoryID: &inventoryID,
			Status:      status,
			Tag:         i % 5, // Tags 0-4
			IsLatest:    true,
			Notes:       fmt.Sprintf("Test sale order %d", i+1),
		})
	}

	return orders
}

// SaleOrderItems creates sale order items for the given sale orders
func SaleOrderItems(saleOrderIDs []uint) []models.SaleOrderItem {
	now := time.Now()
	items := make([]models.SaleOrderItem, 0)

	// Create 2-5 items per sale order
	for i, saleOrderID := range saleOrderIDs {
		numItems := 2 + (i % 4) // 2-5 items per order

		for j := 0; j < numItems; j++ {
			items = append(items, models.SaleOrderItem{
				Base: models.Base{
					CreatedAt: now.Add(-time.Duration(i*10+j) * time.Minute),
					UpdatedAt: now.Add(-time.Duration(i*10+j) * time.Minute),
				},
				SaleOrderID: &saleOrderID,
			})
		}
	}

	return items
}

// createSaleOrderItemMenuRelationships creates many-to-many relationships between sale order items and menu items
func createSaleOrderItemMenuRelationships(tx *gorm.DB, saleOrderItemIDs, menuItemIDs []uint) error {
	type SaleOrderItemMenuItem struct {
		SaleOrderItemID uint `gorm:"primaryKey"`
		MenuItemID      uint `gorm:"primaryKey"`
	}
	var saleOrderItemMenuItems []SaleOrderItemMenuItem

	// Assign 1-3 menu items to each sale order item
	for i, saleOrderItemID := range saleOrderItemIDs {
		numMenuItems := 1 + (i % 3) // 1-3 menu items per sale order item

		for j := 0; j < numMenuItems && j < len(menuItemIDs); j++ {
			// Distribute menu items across sale order items
			menuItemIndex := (i*2 + j) % len(menuItemIDs)
			saleOrderItemMenuItems = append(saleOrderItemMenuItems, SaleOrderItemMenuItem{
				SaleOrderItemID: saleOrderItemID,
				MenuItemID:      menuItemIDs[menuItemIndex],
			})
		}
	}

	if err := tx.Table("sale_order_item_menu_items").Create(&saleOrderItemMenuItems).Error; err != nil {
		return fmt.Errorf("failed to create sale-order-item-menu-item relationships: %w", err)
	}

	return nil
}

// SeedDatabase populates the database with mock data
// SellingPrices generates selling price seed data for the first 5 products.
// Includes multiple effective_from dates to show price history on the timeline.
func SellingPrices(productIDs []uint) []models.SellingPrice {
	var prices []models.SellingPrice

	// Create global selling prices (inventory_id = nil) for up to 5 products
	// with price history (2-3 entries per product)
	priceData := []struct {
		priceHistory []struct {
			price         float64
			effectiveFrom time.Time
		}
	}{
		{priceHistory: []struct {
			price         float64
			effectiveFrom time.Time
		}{
			{price: 15.00, effectiveFrom: time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)},
			{price: 18.00, effectiveFrom: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
			{price: 20.00, effectiveFrom: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		}},
		{priceHistory: []struct {
			price         float64
			effectiveFrom time.Time
		}{
			{price: 25.50, effectiveFrom: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
			{price: 28.00, effectiveFrom: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		}},
		{priceHistory: []struct {
			price         float64
			effectiveFrom time.Time
		}{
			{price: 8.00, effectiveFrom: time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)},
			{price: 9.50, effectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
			{price: 10.00, effectiveFrom: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
		}},
		{priceHistory: []struct {
			price         float64
			effectiveFrom time.Time
		}{
			{price: 45.00, effectiveFrom: time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC)},
			{price: 50.00, effectiveFrom: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)},
		}},
		{priceHistory: []struct {
			price         float64
			effectiveFrom time.Time
		}{
			{price: 12.00, effectiveFrom: time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)},
		}},
	}

	for i, pd := range priceData {
		if i >= len(productIDs) {
			break
		}
		for _, ph := range pd.priceHistory {
			prices = append(prices, models.SellingPrice{
				ProductID:     productIDs[i],
				Price:         decimal.NewFromFloat(ph.price),
				EffectiveFrom: ph.effectiveFrom,
				Notes:         fmt.Sprintf("Seed price for product %d", productIDs[i]),
			})
		}
	}

	return prices
}

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

	// 0. Base Units (Level 1)
	baseUnits := Units()
	if err := tx.Create(&baseUnits).Error; err != nil {
		if postgresErr, ok := err.(*pq.Error); ok && postgresErr.Code != "23505" {
			tx.Rollback()
			return fmt.Errorf("failed to create base units: %w", err)
		}

		log.Println("Base units already exist, skipping creation...")
	}

	// 0.1. Derived Units (Levels 2-4)
	derivedUnits, err := createDerivedUnits(tx, baseUnits)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create derived units: %w", err)
	}

	// 0.2. Build unitIDs map for product creation
	allUnits := append(baseUnits, derivedUnits...)
	unitIDs := make(map[string]uint, len(allUnits))
	for _, unit := range allUnits {
		unitIDs[unit.Symbol] = unit.ID
	}

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
	products := Products(unitIDs)
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

	// 3.5. Selling Prices
	sellingPrices := SellingPrices(productIDs)
	if err := tx.Create(&sellingPrices).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create selling prices: %w", err)
	}

	// 4. MenuItems
	menuItems := MenuItems()
	if err := tx.Create(&menuItems).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create menu items: %w", err)
	}
	// Collect menu item IDs after batch creation
	var menuItemIDs []uint
	for _, menuItem := range menuItems {
		menuItemIDs = append(menuItemIDs, menuItem.ID)
	}

	// 5. Menus
	menus := Menus()
	if err := tx.Create(&menus).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create menus: %w", err)
	}
	// Collect menu IDs after batch creation
	var menuIDs []uint
	for _, menu := range menus {
		menuIDs = append(menuIDs, menu.ID)
	}

	// 5.5. Create menu-item-menu relationships
	if err := createMenuItemMenuRelationships(tx, menuItemIDs, menuIDs); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create menu-item-menu relationships: %w", err)
	}

	// 5.6. Create menu-item-product relationships
	if err := createMenuItemProductRelationships(tx, menuItemIDs, productIDs, products); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create menu-item-product relationships: %w", err)
	}

	// 6. Sale Orders
	// Collect inventory IDs for sale orders
	var inventoryIDs []uint
	for _, inventory := range inventories {
		inventoryIDs = append(inventoryIDs, inventory.ID)
	}
	saleOrders := SaleOrders(inventoryIDs)
	if err := tx.Create(&saleOrders).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create sale orders: %w", err)
	}
	// Collect sale order IDs after batch creation
	var saleOrderIDs []uint
	for _, saleOrder := range saleOrders {
		saleOrderIDs = append(saleOrderIDs, saleOrder.ID)
	}

	// 6.5. Sale Order Items
	saleOrderItems := SaleOrderItems(saleOrderIDs)
	if err := tx.Create(&saleOrderItems).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create sale order items: %w", err)
	}
	// Collect sale order item IDs after batch creation
	var saleOrderItemIDs []uint
	for _, saleOrderItem := range saleOrderItems {
		saleOrderItemIDs = append(saleOrderItemIDs, saleOrderItem.ID)
	}

	// 6.6. Create sale-order-item-menu-item relationships
	if err := createSaleOrderItemMenuRelationships(tx, saleOrderItemIDs, menuItemIDs); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create sale-order-item-menu-item relationships: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Seed users after database schema is populated
	if err := seedUsers(db, cfg.Casbin); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	return nil
}

// seedUsers populates the database with default users
func seedUsers(db *gorm.DB, casbinCfg config.CasbinConfig) error {
	// Initialize Casbin service
	casbinService, err := auth.NewCasbinService(db, casbinCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Casbin service: %w", err)
	}

	// Initialize user repository and service
	userRepo := repository.NewUserRepository(repository.NewBaseRepository(db), "development")
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
		{
			UID:   "demoSeedUidB0000000000000000",
			Email: "cashier@cim.local",
			Name:  "Cashier User",
			Role:  string(models.RoleCashier),
		},
		{
			UID:   "demoSeedUidC0000000000000000",
			Email: "chef@cim.local",
			Name:  "Chef User",
			Role:  string(models.RoleChef),
		},
		{
			UID:   "demoSeedUidA0000000000000000",
			Email: "waiter@cim.local",
			Name:  "Waiter User",
			Role:  string(models.RoleWaiter),
		},
	}

	// Seed each user
	for _, userData := range defaultUsers {
		// Create user
		_, err = userService.CreateUser(ctx, userData.UID, userData.Email, userData.Name, userData.Role, "active")
		if err != nil {
			log.Printf("Failed to create user %s: %v", userData.Email, err)
			continue
		}

		log.Printf("Created user: %s with role: %s", userData.Email, userData.Role)
	}

	log.Println("User seeding completed!")
	return nil
}

// createDerivedUnits creates derived units (levels 2-4) based on the base units
func createDerivedUnits(tx *gorm.DB, baseUnits []models.Unit) ([]models.Unit, error) {
	// Build map for easy lookup of base units by symbol
	unitMap := make(map[string]*models.Unit)
	for i := range baseUnits {
		unitMap[baseUnits[i].Symbol] = &baseUnits[i]
	}

	// Level 2 units
	level2Units := []models.Unit{
		// Mass hierarchy - Level 2: GRAM
		{

			UnitType:         "mass",
			Name:             "GRAM",
			Symbol:           "g",
			ConversionFactor: 0.001,
			Level:            2,
			BaseUnitID:       &unitMap["kg"].ID,
		},
		// Volume hierarchy - Level 2: MILLILITER
		{

			UnitType:         "volume",
			Name:             "MILLILITER",
			Symbol:           "ml",
			ConversionFactor: 0.001,
			Level:            2,
			BaseUnitID:       &unitMap["liter"].ID,
		},
		// Count hierarchies - Level 2
		{

			UnitType:         "count",
			Name:             "BOX_CARTON",
			Symbol:           "box_c",
			ConversionFactor: 0.25,
			Level:            2,
			BaseUnitID:       &unitMap["carton"].ID,
		},
		{

			UnitType:         "count",
			Name:             "MILLILITER_BOTTLE",
			Symbol:           "ml_btl",
			ConversionFactor: 0.001,
			Level:            2,
			BaseUnitID:       &unitMap["bottle"].ID,
		},
		{

			UnitType:         "count",
			Name:             "MILLILITER_CAN",
			Symbol:           "ml_can",
			ConversionFactor: 0.00303,
			Level:            2,
			BaseUnitID:       &unitMap["can"].ID,
		},
		{

			UnitType:         "count",
			Name:             "PIECE_TRAY",
			Symbol:           "piece_t",
			ConversionFactor: 0.08333,
			Level:            2,
			BaseUnitID:       &unitMap["tray"].ID,
		},
		{

			UnitType:         "count",
			Name:             "SLICE",
			Symbol:           "slice",
			ConversionFactor: 0.05,
			Level:            2,
			BaseUnitID:       &unitMap["loaf"].ID,
		},
	}

	if err := tx.Create(&level2Units).Error; err != nil {
		if postgresErr, ok := err.(*pq.Error); ok && postgresErr.Code != "23505" {
			return nil, fmt.Errorf("failed to create level 2 units: %w", err)
		}
		log.Println("Level 2 units already exist, skipping creation...")
	}

	// Add level 2 units to map
	for i := range level2Units {
		unitMap[level2Units[i].Symbol] = &level2Units[i]
	}

	// Level 3 units
	level3Units := []models.Unit{
		// Mass hierarchy - Level 3: MILLIGRAM
		{

			UnitType:         "mass",
			Name:             "MILLIGRAM",
			Symbol:           "mg",
			ConversionFactor: 0.001,
			Level:            3,
			BaseUnitID:       &unitMap["g"].ID,
		},
		// Volume hierarchy - Level 3: MICROLITER
		{

			UnitType:         "volume",
			Name:             "MICROLITER",
			Symbol:           "μl",
			ConversionFactor: 0.001,
			Level:            3,
			BaseUnitID:       &unitMap["ml"].ID,
		},
		// Count hierarchy - Level 3: PACK_CARTON
		{

			UnitType:         "count",
			Name:             "PACK_CARTON",
			Symbol:           "pack_c",
			ConversionFactor: 0.5,
			Level:            3,
			BaseUnitID:       &unitMap["box_c"].ID,
		},
	}

	if err := tx.Create(&level3Units).Error; err != nil {
		if postgresErr, ok := err.(*pq.Error); ok && postgresErr.Code != "23505" {
			return nil, fmt.Errorf("failed to create level 3 units: %w", err)
		}
		log.Println("Level 3 units already exist, skipping creation...")
	}

	// Add level 3 units to map
	for i := range level3Units {
		unitMap[level3Units[i].Symbol] = &level3Units[i]
	}

	// Level 4 units
	level4Units := []models.Unit{
		// Mass hierarchy - Level 4: MICROGRAM
		{

			UnitType:         "mass",
			Name:             "MICROGRAM",
			Symbol:           "mcg",
			ConversionFactor: 0.001,
			Level:            4,
			BaseUnitID:       &unitMap["mg"].ID,
		},
		// Volume hierarchy - Level 4: NANOLITER
		{

			UnitType:         "volume",
			Name:             "NANOLITER",
			Symbol:           "nl",
			ConversionFactor: 0.001,
			Level:            4,
			BaseUnitID:       &unitMap["μl"].ID,
		},
		// Count hierarchy - Level 4: PIECE_CARTON
		{

			UnitType:         "count",
			Name:             "PIECE_CARTON",
			Symbol:           "piece_c",
			ConversionFactor: 0.1,
			Level:            4,
			BaseUnitID:       &unitMap["pack_c"].ID,
		},
	}

	if err := tx.Create(&level4Units).Error; err != nil {
		if postgresErr, ok := err.(*pq.Error); ok && postgresErr.Code != "23505" {
			return nil, fmt.Errorf("failed to create level 4 units: %w", err)
		}
		log.Println("Level 4 units already exist, skipping creation...")
	}

	// Combine all derived units
	allDerived := append(level2Units, level3Units...)
	allDerived = append(allDerived, level4Units...)

	return allDerived, nil
}
