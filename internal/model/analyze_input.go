package model

// AnalyzeInput carries the normalized request plus optional battle report context.
// The service keeps both so prompt builders can use compact request fields and
// the original report context without forcing all callers to depend on raw JSON.
type AnalyzeInput struct {
	Request AnalyzeRequest
	Report  *BattleReport
}
