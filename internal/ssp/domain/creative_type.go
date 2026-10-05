package domain

// CreativeType — тип рекламного материала.
type CreativeType string

// Поддерживаемые типы креативов.
const (
	CreativeTypeBanner CreativeType = "banner"
	CreativeTypeVideo  CreativeType = "video"
	CreativeTypeNative CreativeType = "native"
	CreativeTypeAudio  CreativeType = "audio"
)
