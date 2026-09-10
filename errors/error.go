package error

type UnsupportedLanguageError struct {
	Lang string
}

func (e *UnsupportedLanguageError) Error() string {
	return "unsupported language: " + e.Lang
}
