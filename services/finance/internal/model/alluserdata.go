package model

// AllUserData holds all user data returned by the GetAllUserData RPC.
// Used by the datarights service to export user data.
type AllUserData struct {
	Tags     []*Tag
	Periods  []*BudgetPeriod
	Defaults *DefaultSettings
}
