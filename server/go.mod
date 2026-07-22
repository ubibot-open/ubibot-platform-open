module github.com/ubibot/ubibot-platform-open

go 1.23

require (
	github.com/glebarez/sqlite v1.11.0
	gorm.io/gorm v1.31.2
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.20.0 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)

// gorm.io/* are vanity import paths that resolve via an HTTP meta-tag
// redirect to these same github.com repos. This environment's module
// fetches can reach github.com directly but not the gorm.io redirect (or
// golang.org, modernc.org, gitlab.com — several otherwise-natural
// dependencies live behind hosts this network can't reach), so the
// replace pins each to its real repo instead of the vanity path.
replace (
	golang.org/x/text => github.com/golang/text v0.20.0
	gorm.io/driver/sqlite => github.com/go-gorm/sqlite v1.6.0
	gorm.io/gorm => github.com/go-gorm/gorm v1.31.2
)
