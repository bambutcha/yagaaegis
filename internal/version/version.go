// Package version — единая строка версии для UI, логов и релизов.
package version

// String — SemVer. Менять по правилу .cursor/rules/yagaaegis-versioning.mdc
const String = "1.0.5"

// Author — отображается в UI рядом с версией и в метаданных сборки.
const Author = "bambutcha"

// AuthorURL — ссылка на профиль / репозиторий.
const AuthorURL = "https://github.com/bambutcha"

// Label — для UI («v1.0.4»).
func Label() string {
	return "v" + String
}

// Credit — «v1.0.4 · bambutcha».
func Credit() string {
	return Label() + " · " + Author
}
