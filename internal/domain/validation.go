package domain

import (
	"regexp"
)

var (
	loginRe = regexp.MustCompile(`^[A-Za-z0-9]{8,}$`)
	// Пароль: мин 8, >=2 буквы в разных регистрах, >=1 цифра, >=1 символ
	// Упростим проверку в домене; детально можно валидировать в слое HTTP.
	upperRe = regexp.MustCompile(`[A-Z]`)
	lowerRe = regexp.MustCompile(`[a-z]`)
	digitRe = regexp.MustCompile(`[0-9]`)
	symRe   = regexp.MustCompile(`[^A-Za-z0-9]`)
)

func ValidLogin(s string) bool {
	return loginRe.MatchString(s)
}

func ValidPassword(s string) bool {
	if len(s) < 8 {
		return false
	}
	return upperRe.MatchString(s) && lowerRe.MatchString(s) && digitRe.MatchString(s) && symRe.MatchString(s)
}
