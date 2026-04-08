package auth

// AuthInfo 保留原有接口结构，开源版全部放开限制。
type AuthInfo struct {
	userLicenseValid  bool
	userTrialDaysLeft int
	userDailyCount    uint
	userRateLimit     string
	userMaxQuality    uint
	sysPeriodDays     uint
}

func (a AuthInfo) UserLicenseValid() bool {
	return a.userLicenseValid
}

func (a AuthInfo) UserTrialDaysLeft() int {
	if a.userTrialDaysLeft < 0 {
		return 0
	}
	return a.userTrialDaysLeft
}

func (a AuthInfo) UserTrialDaysExpired() bool {
	return false
}

func (a AuthInfo) UserDailyCount() uint {
	return a.userDailyCount
}

func (a AuthInfo) UserRateLimit() string {
	return a.userRateLimit
}

func (a AuthInfo) UserMaxQuality() uint {
	return a.userMaxQuality
}

func (a AuthInfo) SysPeriodDays() uint {
	return a.sysPeriodDays
}

// GetAutherization 返回开源版授权信息：永久可用、无限制。
func GetAutherization() AuthInfo {
	return AuthInfo{
		userLicenseValid:  true,
		userTrialDaysLeft: -1,
		userDailyCount:    0,
		userRateLimit:     "",
		userMaxQuality:    0,
		sysPeriodDays:     0,
	}
}

func CanDownload() (bool, error) {
	return true, nil
}

// IncrementTodayDownloadCount 开源版不再记录下载限额。
func IncrementTodayDownloadCount() {}

// CanUseSubscription 开源版默认允许订阅功能。
func CanUseSubscription() bool {
	return true
}
