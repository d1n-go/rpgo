package rpgo

// MailType — поле "type" в CommonInfo: заказное или простое отправление.
type MailType string

const (
	MailTypeRegistered MailType = "REGISTERED"
	MailTypeRegular    MailType = "REGULAR"
)

// PostMark — битовая маска "Код отметок", управляющая особой обработкой
// отправления (невозвратное, с электронным уведомлением о вручении, с
// юридически значимым электронным уведомлением о вручении / ЮЗЭУВ).
type PostMark int64

const (
	PostMarkNone                                           PostMark = 0
	PostMarkNoReturn                                       PostMark = 1024
	PostMarkWithDeliveryNotice                             PostMark = 16384
	PostMarkNoReturnWithDeliveryNotice                     PostMark = 17408
	PostMarkWithLegallySignificantElectronicNotice         PostMark = 2199023255552
	PostMarkNoReturnWithLegallySignificantElectronicNotice PostMark = 2199023256576
)

// CommonInfo — часть "commonInfo" multipart-запроса POST /letters: метаданные
// письма, передаются JSON-частью вместе с PDF/XML/картинками-вложениями и их
// отсоединёнными подписями.
//
// С сентября 2024 года сведения о машиночитаемой доверенности (МЧД)
// передаются как обычные текстовые записи в Attributes — "m4d.reference" и
// "m4d.info", отдельных полей под них в API нет (см. документацию, раздел
// «Передача данных о машиночитаемой доверенности»).
type CommonInfo struct {
	// mailID — ШПИ (штриховой почтовый идентификатор). Оставьте пустым,
	// чтобы Почта России присвоила его сама — если только для вашей
	// интеграции явно не согласовано присвоение ШПИ на своей стороне (см.
	// примечание в документации API к полю mailId).
	MailID          string            `json:"mailId,omitempty"`
	MailRank        int               `json:"mailRank,omitempty"`
	Type            MailType          `json:"type"`
	TemplateID      string            `json:"templateId,omitempty"`
	Attachments     Attachments       `json:"attachments"`
	Document        Document          `json:"document"`
	Charge          *Charge           `json:"charge,omitempty"`
	Sender          Sender            `json:"sender"`
	Recipient       Recipient         `json:"recipient"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	PrintAttributes map[string]string `json:"printAttributes,omitempty"`
	OrderFormNumber string            `json:"orderFormNumber,omitempty"`
	PostMark        PostMark          `json:"postMark,omitempty"`
}

// AttachmentRef ссылается на блок multipart-запроса по имени поля формы, а
// также на имя поля с отсоединённой электронной подписью в формате
// PKCS#7/DER — см. в документации API раздел «Требования к файлам ЭП».
type AttachmentRef struct {
	AttachmentID          string `json:"attachmentId"`
	AttachmentSignatureID string `json:"attachmentSignatureId,omitempty"`
}

// Attachments описывает содержимое письма: должно быть задано хотя бы одно
// из полей PDF или XML.
type Attachments struct {
	Images            []AttachmentRef `json:"images,omitempty"`
	PDF               *AttachmentRef  `json:"pdf,omitempty"`
	OnlineDeliveryPDF *AttachmentRef  `json:"onlineDeliveryPdf,omitempty"`
	XML               *AttachmentRef  `json:"xml,omitempty"`
}

// Document описывает исходящий электронный документ, который несёт письмо.
type Document struct {
	ID                 string `json:"id"`
	RegistrationDate   Time   `json:"registrationDate"`
	RegistrationNumber string `json:"registrationNumber"`
	Title              string `json:"title,omitempty"`
}

// Charge — реквизиты для перечисления денежных средств (например, штрафа),
// связанного с письмом.
type Charge struct {
	BankName         string `json:"bankName"`
	CorrAccount      string `json:"corrAccount,omitempty"`
	OperatingAccount string `json:"operatingAccount"`
	BIK              string `json:"bik"`
	INN              string `json:"inn"`
	KPP              string `json:"kpp"`
	KBK              string `json:"kbk"`
	OKTMO            string `json:"oktmo"`
	PaymentType      string `json:"paymentType,omitempty"`
	PaymentName      string `json:"paymentName"`
	RecipientName    string `json:"recipientName"`
	UIN              string `json:"uin"`
	// AmountKopecks — сумма в копейках, как описано в API ("Сумма в
	// копейках").
	AmountKopecks  int64  `json:"amount"`
	PaymentPeriod  int    `json:"paymentPeriod"`
	AdditionalInfo string `json:"additionalInfo,omitempty"`
}

// Sender идентифицирует отправителя письма. Code соответствует
// документированному формату "CODE_${CUSTOMER-ID}".
type Sender struct {
	Code           string         `json:"code"`
	Name           string         `json:"name"`
	DepartmentCode string         `json:"departmentCode"`
	DepartmentName string         `json:"departmentName"`
	PersonCode     string         `json:"personCode,omitempty"`
	PersonName     string         `json:"personName,omitempty"`
	Address        string         `json:"address,omitempty"`
	AddressStruct  *AddressStruct `json:"addressStruct,omitempty"`
	ReturnAddress  *ReturnAddress `json:"returnAddress,omitempty"`
}

// ReturnAddress — адрес, на который возвращается недоставленное отправление.
type ReturnAddress struct {
	ZipCode   string `json:"zipCode,omitempty"`
	Region    string `json:"region,omitempty"`
	District  string `json:"district,omitempty"`
	City      string `json:"city,omitempty"`
	Locality  string `json:"locality,omitempty"`
	Street    string `json:"street,omitempty"`
	House     string `json:"house,omitempty"`
	Building  string `json:"building,omitempty"`
	Apartment string `json:"apartment,omitempty"`
}

// Recipient идентифицирует получателя письма: должно быть задано ровно одно
// из полей Person или Organization, а также Address либо AddressStruct.
type Recipient struct {
	// countryCodeOKSM — код страны получателя по ОКСМ; 643 — Россия. Любое
	// другое значение делает отправление международным.
	CountryCodeOKSM int            `json:"countryCodeOKSM"`
	Address         string         `json:"address,omitempty"`
	AddressStruct   *AddressStruct `json:"addressStruct,omitempty"`
	Person          *Person        `json:"person,omitempty"`
	Organization    *Organization  `json:"organization,omitempty"`
}

// AddressStruct — структурированная форма адреса, альтернатива строковому
// Address.
type AddressStruct struct {
	City           string `json:"city,omitempty"`
	CityType       string `json:"cityType,omitempty"`
	District       string `json:"district,omitempty"`
	Entrance       string `json:"entrance,omitempty"`
	Flat           string `json:"flat,omitempty"`
	Floor          string `json:"floor,omitempty"`
	Hostel         string `json:"hostel,omitempty"`
	HouseNumber    string `json:"houseNumber,omitempty"`
	Corpus         string `json:"corpus,omitempty"`
	MilitaryNumber string `json:"militaryNumber,omitempty"`
	Office         string `json:"office,omitempty"`
	OKATOCode      string `json:"okatoCode,omitempty"`
	PostalBox      string `json:"postalBox,omitempty"`
	PostalCode     string `json:"postalCode,omitempty"`
	Area           string `json:"area,omitempty"`
	AreaType       string `json:"areaType,omitempty"`
	Region         string `json:"region,omitempty"`
	RegionType     string `json:"regionType,omitempty"`
	Room           string `json:"room,omitempty"`
	Section        string `json:"section,omitempty"`
	Settlement     string `json:"settlement,omitempty"`
	SettlementType string `json:"settlementType,omitempty"`
	Street         string `json:"street,omitempty"`
	StreetType     string `json:"streetType,omitempty"`
	Stroenie       string `json:"stroenie,omitempty"`
	Vladenie       string `json:"vladenie,omitempty"`
}

// Person — получатель-физическое лицо.
type Person struct {
	FirstName        string            `json:"firstName"`
	Patronymic       string            `json:"patronymic,omitempty"`
	LastName         string            `json:"lastName"`
	Birthdate        string            `json:"birthdate,omitempty"`
	IdentityDocument *IdentityDocument `json:"identityDocument,omitempty"`
	INN              string            `json:"inn,omitempty"`
	SNILS            string            `json:"snils,omitempty"`
	DrivingLicence   *DrivingLicense   `json:"drivingLicence,omitempty"`
	OGRNIP           string            `json:"ogrnip,omitempty"`
	VehicleRegDoc    *VehicleRegDoc    `json:"vehicleRegDoc,omitempty"`
}

// IdentityDocument — документ, удостоверяющий личность получателя (например,
// паспорт).
type IdentityDocument struct {
	Series    string `json:"series"`
	Number    string `json:"number"`
	IssueDate Time   `json:"issueDate"`
	Issuer    string `json:"issuer"`
}

// DrivingLicense — реквизиты водительского удостоверения получателя.
type DrivingLicense struct {
	Series string `json:"series"`
	Number string `json:"number"`
}

// VehicleRegDoc — реквизиты свидетельства о регистрации транспортного
// средства получателя.
type VehicleRegDoc struct {
	Series string `json:"series"`
	Number string `json:"number"`
}

// Organization — получатель-юридическое лицо.
type Organization struct {
	Name string `json:"name"`
	INN  string `json:"inn,omitempty"`
	KPP  string `json:"kpp,omitempty"`
	OGRN string `json:"ogrn,omitempty"`
}
