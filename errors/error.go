package error

type UnsupportedLanguageError struct {
	Lang string
}

func (e *UnsupportedLanguageError) Error() string {
	return "unsupported language: " + e.Lang
}

func NewUnsupportedLanguageError(lang string) *UnsupportedLanguageError {
	return &UnsupportedLanguageError{Lang: lang}
}
