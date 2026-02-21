package moneyeu

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"Shopify-GreenMoney-Lockout/internal/email"
)

type ShopifyOrderLite struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	TotalPrice          string   `json:"total_price"`
	Currency            string   `json:"currency"`
	PaymentGatewayNames []string `json:"payment_gateway_names"`

	BillingAddress  *ShopifyAddressLite `json:"billing_address"`
	ShippingAddress *ShopifyAddressLite `json:"shipping_address"`
}

type ShopifyAddressLite struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone"`
	Address1    string `json:"address1"`
	City        string `json:"city"`
	Province    string `json:"province"`
	Zip         string `json:"zip"`
	CountryCode string `json:"country_code"`
}

type Service struct {
	DB     *sql.DB
	Client *Client
	SMTP   email.SMTPConfig
}

func (s *Service) HandleShopifyOrderJSON(ctx context.Context, raw []byte) error {
	var o ShopifyOrderLite
	if err := json.Unmarshal(raw, &o); err != nil {
		return fmt.Errorf("moneyeu: decode Shopify order: %w", err)
	}

	amount, err := strconv.ParseFloat(o.TotalPrice, 64)
	if err != nil {
		return fmt.Errorf("parse total_price %q: %w", o.TotalPrice, err)
	}

	// choose address
	addr := o.ShippingAddress
	if addr == nil {
		addr = o.BillingAddress
	}

	customerName := ""
	customerPhone := ""
	address1 := ""
	city := ""
	state := ""
	zip := ""
	country := ""

	if addr != nil {
		customerPhone = addr.Phone
		address1 = addr.Address1
		city = addr.City
		state = addr.Province
		zip = addr.Zip
		country = addr.CountryCode
		customerName = strings.TrimSpace(addr.FirstName + " " + addr.LastName)
	}
	// MoneyEU API Expects X, Shopify sends down Y, update dial code for non North American orders as well
	dialCode := "+1"

	switch country {
	// A
	case "AF":
		country = "Afghanistan"
		dialCode = "+93"
	case "AX":
		country = "Åland Islands"
		dialCode = "+358"
	case "AL":
		country = "Albania"
		dialCode = "+355"
	case "DZ":
		country = "Algeria"
		dialCode = "+213"
	case "AS":
		country = "American Samoa"
	case "AD":
		country = "Andorra"
		dialCode = "+376"
	case "AO":
		country = "Angola"
		dialCode = "+244"
	case "AI":
		country = "Anguilla"
	case "AQ":
		country = "Antarctica"
		dialCode = "+672"
	case "AG":
		country = "Antigua and Barbuda"
	case "AR":
		country = "Argentina"
		dialCode = "+54"
	case "AM":
		country = "Armenia"
		dialCode = "+374"
	case "AW":
		country = "Aruba"
		dialCode = "+297"
	case "AU":
		country = "Australia"
		dialCode = "+61"
	case "AT":
		country = "Austria"
		dialCode = "+43"
	case "AZ":
		country = "Azerbaijan"
		dialCode = "+994"

	// B
	case "BS":
		country = "Bahamas"
	case "BH":
		country = "Bahrain"
		dialCode = "+973"
	case "BD":
		country = "Bangladesh"
		dialCode = "+880"
	case "BB":
		country = "Barbados"
	case "BY":
		country = "Belarus"
		dialCode = "+375"
	case "BE":
		country = "Belgium"
		dialCode = "+32"
	case "BZ":
		country = "Belize"
		dialCode = "+501"
	case "BJ":
		country = "Benin"
		dialCode = "+229"
	case "BM":
		country = "Bermuda"
	case "BT":
		country = "Bhutan"
		dialCode = "+975"
	case "BO":
		country = "Bolivia"
		dialCode = "+591"
	case "BA":
		country = "Bosnia and Herzegovina"
		dialCode = "+387"
	case "BW":
		country = "Botswana"
		dialCode = "+267"
	case "BV":
		country = "Bouvet Island"
		dialCode = "+47"
	case "BR":
		country = "Brazil"
		dialCode = "+55"
	case "IO":
		country = "British Indian Ocean Territory"
		dialCode = "+246"
	case "BN":
		country = "Brunei Darussalam"
		dialCode = "+673"
	case "BG":
		country = "Bulgaria"
		dialCode = "+359"
	case "BF":
		country = "Burkina Faso"
		dialCode = "+226"
	case "BI":
		country = "Burundi"
		dialCode = "+257"

	// C
	case "KH":
		country = "Cambodia"
		dialCode = "+855"
	case "CM":
		country = "Cameroon"
		dialCode = "+237"
	case "CA":
		country = "Canada"
	case "CV":
		country = "Cape Verde"
		dialCode = "+238"
	case "KY":
		country = "Cayman Islands"
	case "CF":
		country = "Central African Republic"
		dialCode = "+236"
	case "TD":
		country = "Chad"
		dialCode = "+235"
	case "CL":
		country = "Chile"
		dialCode = "+56"
	case "CN":
		country = "China"
		dialCode = "+86"
	case "CX":
		country = "Christmas Island"
		dialCode = "+61"
	case "CC":
		country = "Cocos (Keeling) Islands"
		dialCode = "+61"
	case "CO":
		country = "Colombia"
		dialCode = "+57"
	case "KM":
		country = "Comoros"
		dialCode = "+269"
	case "CG":
		country = "Congo"
		dialCode = "+242"
	case "CD":
		country = "Congo, The Democratic Republic of the"
		dialCode = "+243"
	case "CK":
		country = "Cook Islands"
		dialCode = "+682"
	case "CR":
		country = "Costa Rica"
		dialCode = "+506"
	case "CI":
		country = "Côte d'Ivoire"
		dialCode = "+225"
	case "HR":
		country = "Croatia"
		dialCode = "+385"
	case "CU":
		country = "Cuba"
		dialCode = "+53"
	case "CY":
		country = "Cyprus"
		dialCode = "+357"
	case "CZ":
		country = "Czech Republic"
		dialCode = "+420"

	// D
	case "DK":
		country = "Denmark"
		dialCode = "+45"
	case "DJ":
		country = "Djibouti"
		dialCode = "+253"
	case "DM":
		country = "Dominica"
	case "DO":
		country = "Dominican Republic"

	// E
	case "EC":
		country = "Ecuador"
		dialCode = "+593"
	case "EG":
		country = "Egypt"
		dialCode = "+20"
	case "SV":
		country = "El Salvador"
		dialCode = "+503"
	case "GQ":
		country = "Equatorial Guinea"
		dialCode = "+240"
	case "ER":
		country = "Eritrea"
		dialCode = "+291"
	case "EE":
		country = "Estonia"
		dialCode = "+372"
	case "ET":
		country = "Ethiopia"
		dialCode = "+251"

	// F
	case "FK":
		country = "Falkland Islands (Malvinas)"
		dialCode = "+500"
	case "FO":
		country = "Faroe Islands"
		dialCode = "+298"
	case "FJ":
		country = "Fiji"
		dialCode = "+679"
	case "FI":
		country = "Finland"
		dialCode = "+358"
	case "FR":
		country = "France"
		dialCode = "+33"

	// G
	case "GA":
		country = "Gabon"
		dialCode = "+241"
	case "GM":
		country = "Gambia"
		dialCode = "+220"
	case "GE":
		country = "Georgia"
		dialCode = "+995"
	case "DE":
		country = "Germany"
		dialCode = "+49"
	case "GH":
		country = "Ghana"
		dialCode = "+233"
	case "GI":
		country = "Gibraltar"
		dialCode = "+350"
	case "GR":
		country = "Greece"
		dialCode = "+30"
	case "GL":
		country = "Greenland"
		dialCode = "+299"
	case "GD":
		country = "Grenada"
	case "GP":
		country = "Guadeloupe"
		dialCode = "+590"
	case "GU":
		country = "Guam"
	case "GT":
		country = "Guatemala"
		dialCode = "+502"
	case "GG":
		country = "Guernsey"
		dialCode = "+44"
	case "GN":
		country = "Guinea"
		dialCode = "+224"
	case "GW":
		country = "Guinea-Bissau"
		dialCode = "+245"
	case "GY":
		country = "Guyana"
		dialCode = "+592"

	// H
	case "HT":
		country = "Haiti"
		dialCode = "+509"
	case "HM":
		country = "Heard Island and McDonald Islands"
		dialCode = "+672"
	case "VA":
		country = "Holy See (Vatican City State)"
		dialCode = "+379"
	case "HN":
		country = "Honduras"
		dialCode = "+504"
	case "HK":
		country = "Hong Kong"
		dialCode = "+852"
	case "HU":
		country = "Hungary"
		dialCode = "+36"

	// I
	case "IS":
		country = "Iceland"
		dialCode = "+354"
	case "IN":
		country = "India"
		dialCode = "+91"
	case "ID":
		country = "Indonesia"
		dialCode = "+62"
	case "IR":
		country = "Iran, Islamic Republic of"
		dialCode = "+98"
	case "IQ":
		country = "Iraq"
		dialCode = "+964"
	case "IE":
		country = "Ireland"
		dialCode = "+353"
	case "IM":
		country = "Isle of Man"
		dialCode = "+44"
	case "IL":
		country = "Israel"
		dialCode = "+972"
	case "IT":
		country = "Italy"
		dialCode = "+39"

	// J
	case "JM":
		country = "Jamaica"
	case "JP":
		country = "Japan"
		dialCode = "+81"
	case "JE":
		country = "Jersey"
		dialCode = "+44"
	case "JO":
		country = "Jordan"
		dialCode = "+962"

	// K
	case "KZ":
		country = "Kazakhstan"
		dialCode = "+7"
	case "KE":
		country = "Kenya"
		dialCode = "+254"
	case "KI":
		country = "Kiribati"
		dialCode = "+686"
	case "KP":
		country = "Korea, Democratic People's Republic of"
		dialCode = "+850"
	case "KR":
		country = "Korea, Republic of"
		dialCode = "+82"
	case "KW":
		country = "Kuwait"
		dialCode = "+965"
	case "KG":
		country = "Kyrgyzstan"
		dialCode = "+996"

	// L
	case "LA":
		country = "Lao People's Democratic Republic"
		dialCode = "+856"
	case "LV":
		country = "Latvia"
		dialCode = "+371"
	case "LB":
		country = "Lebanon"
		dialCode = "+961"
	case "LS":
		country = "Lesotho"
		dialCode = "+266"
	case "LR":
		country = "Liberia"
		dialCode = "+231"
	case "LY":
		country = "Libya"
		dialCode = "+218"
	case "LI":
		country = "Liechtenstein"
		dialCode = "+423"
	case "LT":
		country = "Lithuania"
		dialCode = "+370"
	case "LU":
		country = "Luxembourg"
		dialCode = "+352"

	// M
	case "MO":
		country = "Macao"
		dialCode = "+853"
	case "MK":
		country = "North Macedonia"
		dialCode = "+389"
	case "MG":
		country = "Madagascar"
		dialCode = "+261"
	case "MW":
		country = "Malawi"
		dialCode = "+265"
	case "MY":
		country = "Malaysia"
		dialCode = "+60"
	case "MV":
		country = "Maldives"
		dialCode = "+960"
	case "ML":
		country = "Mali"
		dialCode = "+223"
	case "MT":
		country = "Malta"
		dialCode = "+356"
	case "MH":
		country = "Marshall Islands"
		dialCode = "+692"
	case "MQ":
		country = "Martinique"
		dialCode = "+596"
	case "MR":
		country = "Mauritania"
		dialCode = "+222"
	case "MU":
		country = "Mauritius"
		dialCode = "+230"
	case "YT":
		country = "Mayotte"
		dialCode = "+262"
	case "MX":
		country = "Mexico"
		dialCode = "+52"
	case "FM":
		country = "Micronesia, Federated States of"
		dialCode = "+691"
	case "MD":
		country = "Moldova, Republic of"
		dialCode = "+373"
	case "MC":
		country = "Monaco"
		dialCode = "+377"
	case "MN":
		country = "Mongolia"
		dialCode = "+976"
	case "MS":
		country = "Montserrat"
	case "MA":
		country = "Morocco"
		dialCode = "+212"
	case "MZ":
		country = "Mozambique"
		dialCode = "+258"
	case "MM":
		country = "Myanmar"
		dialCode = "+95"

	// N
	case "NA":
		country = "Namibia"
		dialCode = "+264"
	case "NR":
		country = "Nauru"
		dialCode = "+674"
	case "NP":
		country = "Nepal"
		dialCode = "+977"
	case "NL":
		country = "Netherlands"
		dialCode = "+31"
	case "NC":
		country = "New Caledonia"
		dialCode = "+687"
	case "NZ":
		country = "New Zealand"
		dialCode = "+64"
	case "NI":
		country = "Nicaragua"
		dialCode = "+505"
	case "NE":
		country = "Niger"
		dialCode = "+227"
	case "NG":
		country = "Nigeria"
		dialCode = "+234"
	case "NU":
		country = "Niue"
		dialCode = "+683"
	case "NF":
		country = "Norfolk Island"
		dialCode = "+672"
	case "MP":
		country = "Northern Mariana Islands"
	case "NO":
		country = "Norway"
		dialCode = "+47"

	// O
	case "OM":
		country = "Oman"
		dialCode = "+968"

	// P
	case "PK":
		country = "Pakistan"
		dialCode = "+92"
	case "PW":
		country = "Palau"
		dialCode = "+680"
	case "PS":
		country = "Palestinian Territory, Occupied"
		dialCode = "+970"
	case "PA":
		country = "Panama"
		dialCode = "+507"
	case "PG":
		country = "Papua New Guinea"
		dialCode = "+675"
	case "PY":
		country = "Paraguay"
		dialCode = "+595"
	case "PE":
		country = "Peru"
		dialCode = "+51"
	case "PH":
		country = "Philippines"
		dialCode = "+63"
	case "PN":
		country = "Pitcairn"
		dialCode = "+64"
	case "PL":
		country = "Poland"
		dialCode = "+48"
	case "PT":
		country = "Portugal"
		dialCode = "+351"
	case "PR":
		country = "Puerto Rico"

	// Q
	case "QA":
		country = "Qatar"
		dialCode = "+974"

	// R
	case "RE":
		country = "Reunion"
		dialCode = "+262"
	case "RO":
		country = "Romania"
		dialCode = "+40"
	case "RU":
		country = "Russian Federation"
		dialCode = "+7"
	case "RW":
		country = "Rwanda"
		dialCode = "+250"

	// S
	case "SH":
		country = "Saint Helena"
		dialCode = "+290"
	case "KN":
		country = "Saint Kitts and Nevis"
	case "LC":
		country = "Saint Lucia"
	case "PM":
		country = "Saint Pierre and Miquelon"
		dialCode = "+508"
	case "VC":
		country = "Saint Vincent and the Grenadines"
	case "WS":
		country = "Samoa"
		dialCode = "+685"
	case "SM":
		country = "San Marino"
		dialCode = "+378"
	case "ST":
		country = "Sao Tome and Principe"
		dialCode = "+239"
	case "SA":
		country = "Saudi Arabia"
		dialCode = "+966"
	case "SN":
		country = "Senegal"
		dialCode = "+221"
	case "RS":
		country = "Serbia"
		dialCode = "+381"
	case "SC":
		country = "Seychelles"
		dialCode = "+248"
	case "SL":
		country = "Sierra Leone"
		dialCode = "+232"
	case "SG":
		country = "Singapore"
		dialCode = "+65"
	case "SK":
		country = "Slovakia"
		dialCode = "+421"
	case "SI":
		country = "Slovenia"
		dialCode = "+386"
	case "SB":
		country = "Solomon Islands"
		dialCode = "+677"
	case "SO":
		country = "Somalia"
		dialCode = "+252"
	case "ZA":
		country = "South Africa"
		dialCode = "+27"
	case "GS":
		country = "South Georgia and the South Sandwich Islands"
		dialCode = "+500"
	case "ES":
		country = "Spain"
		dialCode = "+34"
	case "LK":
		country = "Sri Lanka"
		dialCode = "+94"
	case "SD":
		country = "Sudan"
		dialCode = "+249"
	case "SR":
		country = "Suriname"
		dialCode = "+597"
	case "SJ":
		country = "Svalbard and Jan Mayen"
		dialCode = "+47"
	case "SZ":
		country = "Eswatini"
		dialCode = "+268"
	case "SE":
		country = "Sweden"
		dialCode = "+46"
	case "CH":
		country = "Switzerland"
		dialCode = "+41"
	case "SY":
		country = "Syrian Arab Republic"
		dialCode = "+963"

	// T
	case "TW":
		country = "Taiwan"
		dialCode = "+886"
	case "TJ":
		country = "Tajikistan"
		dialCode = "+992"
	case "TZ":
		country = "Tanzania"
		dialCode = "+255"
	case "TH":
		country = "Thailand"
		dialCode = "+66"
	case "TL":
		country = "Timor-Leste"
		dialCode = "+670"
	case "TG":
		country = "Togo"
		dialCode = "+228"
	case "TK":
		country = "Tokelau"
		dialCode = "+690"
	case "TO":
		country = "Tonga"
		dialCode = "+676"
	case "TT":
		country = "Trinidad and Tobago"
	case "TN":
		country = "Tunisia"
		dialCode = "+216"
	case "TR":
		country = "Turkey"
		dialCode = "+90"
	case "TM":
		country = "Turkmenistan"
		dialCode = "+993"
	case "TC":
		country = "Turks and Caicos Islands"
	case "TV":
		country = "Tuvalu"
		dialCode = "+688"

	// U
	case "UG":
		country = "Uganda"
		dialCode = "+256"
	case "UA":
		country = "Ukraine"
		dialCode = "+380"
	case "AE":
		country = "United Arab Emirates"
		dialCode = "+971"
	case "GB":
		country = "United Kingdom"
		dialCode = "+44"
	case "US":
		country = "United States"
	case "UM":
		country = "United States Minor Outlying Islands"
	case "UY":
		country = "Uruguay"
		dialCode = "+598"
	case "UZ":
		country = "Uzbekistan"
		dialCode = "+998"

	// V
	case "VU":
		country = "Vanuatu"
		dialCode = "+678"
	case "VE":
		country = "Venezuela"
		dialCode = "+58"
	case "VN":
		country = "Viet Nam"
		dialCode = "+84"
	case "VG":
		country = "Virgin Islands, British"
	case "VI":
		country = "Virgin Islands, U.S."

	// W
	case "WF":
		country = "Wallis and Futuna"
		dialCode = "+681"
	case "EH":
		country = "Western Sahara"
		dialCode = "+212"

	// Y
	case "YE":
		country = "Yemen"
		dialCode = "+967"

	// Z
	case "ZM":
		country = "Zambia"
		dialCode = "+260"
	case "ZW":
		country = "Zimbabwe"
		dialCode = "+263"

	}

	//*****COMMENT THIS OUT FOR PRODUCTION************
	//o.Currency = "EUR"
	// 1) Insert DB row first
	paymentID, err := InsertMoneyEUPayment(s.DB, PaymentRow{
		ShopifyOrderID:   strconv.FormatInt(o.ID, 10),
		ShopifyOrderName: o.Name,
		Amount:           amount,
		Currency:         o.Currency,
		CustomerEmail:    o.Email,
		CustomerName:     customerName,
		CustomerPhone:    customerPhone,
	})
	if err != nil {
		return err
	}
	log.Printf("MoneyEU: inserted payment id=%d for order %s", paymentID, o.Name)
	// 2) Create MoneyEU order
	req := CreateOrderExtRequest{
		Amount:          amount,
		Currency:        o.Currency,
		Name:            fallback(customerName, "Customer"),
		Mail:            o.Email,
		PhoneNumber:     customerPhone,
		DialCode:        dialCode,
		Address:         address1,
		City:            city,
		State:           state,
		Zip:             zip,
		Country:         country,
		IdOrderExt:      strconv.FormatInt(o.ID, 10),
		Language:        "English",
		Sms:             false,
		CustomerService: "Lockout Supplements",
	}

	resp, err := s.Client.CreateOrderExt(ctx, req)
	if err != nil {
		return fmt.Errorf("CreateOrderExt: %w", err)
	}

	c, err := resp.FirstContent()
	if err != nil {
		return fmt.Errorf("CreateOrderExt: parse content: %w", err)
	}
	log.Println("MoneyEU: CreateOrderExt response content:", c.ID, c.IdOrderExt, c.Url, c.Status)

	moneyEUOrderID := fmt.Sprintf("%d", c.ID)
	checkoutURL := c.Url
	status := c.Status

	if err := SetMoneyEUOrderLink(s.DB, paymentID, moneyEUOrderID, c.IdOrderExt, checkoutURL, status); err != nil {
		return err
	}

	// 3) Email checkout link
	/*
		subject := fmt.Sprintf("Complete your payment for Order %s", o.Name)
		body := fmt.Sprintf(
			"Hi,\n\nThanks for your order with Lockout Supplements (%s).\n\n"+
				"To complete payment, use the secure checkout link below:\n%s\n\n"+
				"Amount due: %.2f %s\n\n"+
				"If you have any issues, reply to this email and we’ll help.\n\n"+
				"— Lockout Supplements\n",
			o.Name, checkoutURL, amount, o.Currency,
		)

		if err := email.Send(s.SMTP, o.Email, subject, body); err != nil {
			_ = MarkEmailFailed(s.DB, paymentID, err.Error())
			return fmt.Errorf("email send: %w", err)
		}
		_ = MarkEmailSent(s.DB, paymentID)

		log.Printf("MoneyEU: emailed checkout link for order %s", o.Name)
		return nil
	*/
	return nil
}

func fallback(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
