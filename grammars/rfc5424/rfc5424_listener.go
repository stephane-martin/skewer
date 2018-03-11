// Code generated from grammars/rfc5424/RFC5424.g4 by ANTLR 4.7.1. DO NOT EDIT.

package rfc5424 // RFC5424
import "github.com/antlr/antlr4/runtime/Go/antlr"

// RFC5424Listener is a complete listener for a parse tree produced by RFC5424Parser.
type RFC5424Listener interface {
	antlr.ParseTreeListener

	// EnterFull is called when entering the full production.
	EnterFull(c *FullContext)

	// EnterHeadr is called when entering the headr production.
	EnterHeadr(c *HeadrContext)

	// EnterMsg is called when entering the msg production.
	EnterMsg(c *MsgContext)

	// EnterAllchars is called when entering the allchars production.
	EnterAllchars(c *AllcharsContext)

	// EnterTimestamp is called when entering the timestamp production.
	EnterTimestamp(c *TimestampContext)

	// EnterDate is called when entering the date production.
	EnterDate(c *DateContext)

	// EnterYear is called when entering the year production.
	EnterYear(c *YearContext)

	// EnterMonth is called when entering the month production.
	EnterMonth(c *MonthContext)

	// EnterDay is called when entering the day production.
	EnterDay(c *DayContext)

	// EnterTime is called when entering the time production.
	EnterTime(c *TimeContext)

	// EnterHour is called when entering the hour production.
	EnterHour(c *HourContext)

	// EnterMinute is called when entering the minute production.
	EnterMinute(c *MinuteContext)

	// EnterSecond is called when entering the second production.
	EnterSecond(c *SecondContext)

	// EnterNano is called when entering the nano production.
	EnterNano(c *NanoContext)

	// EnterTimezone is called when entering the timezone production.
	EnterTimezone(c *TimezoneContext)

	// EnterTimezonenum is called when entering the timezonenum production.
	EnterTimezonenum(c *TimezonenumContext)

	// EnterPri is called when entering the pri production.
	EnterPri(c *PriContext)

	// EnterVersion is called when entering the version production.
	EnterVersion(c *VersionContext)

	// EnterHostname is called when entering the hostname production.
	EnterHostname(c *HostnameContext)

	// EnterAppname is called when entering the appname production.
	EnterAppname(c *AppnameContext)

	// EnterMsgid is called when entering the msgid production.
	EnterMsgid(c *MsgidContext)

	// EnterProcid is called when entering the procid production.
	EnterProcid(c *ProcidContext)

	// EnterAllascii is called when entering the allascii production.
	EnterAllascii(c *AllasciiContext)

	// EnterStructured is called when entering the structured production.
	EnterStructured(c *StructuredContext)

	// EnterElement is called when entering the element production.
	EnterElement(c *ElementContext)

	// EnterSid is called when entering the sid production.
	EnterSid(c *SidContext)

	// EnterParam is called when entering the param production.
	EnterParam(c *ParamContext)

	// EnterName is called when entering the name production.
	EnterName(c *NameContext)

	// EnterSpecialname is called when entering the specialname production.
	EnterSpecialname(c *SpecialnameContext)

	// EnterValue is called when entering the value production.
	EnterValue(c *ValueContext)

	// EnterSpecialvalue is called when entering the specialvalue production.
	EnterSpecialvalue(c *SpecialvalueContext)

	// ExitFull is called when exiting the full production.
	ExitFull(c *FullContext)

	// ExitHeadr is called when exiting the headr production.
	ExitHeadr(c *HeadrContext)

	// ExitMsg is called when exiting the msg production.
	ExitMsg(c *MsgContext)

	// ExitAllchars is called when exiting the allchars production.
	ExitAllchars(c *AllcharsContext)

	// ExitTimestamp is called when exiting the timestamp production.
	ExitTimestamp(c *TimestampContext)

	// ExitDate is called when exiting the date production.
	ExitDate(c *DateContext)

	// ExitYear is called when exiting the year production.
	ExitYear(c *YearContext)

	// ExitMonth is called when exiting the month production.
	ExitMonth(c *MonthContext)

	// ExitDay is called when exiting the day production.
	ExitDay(c *DayContext)

	// ExitTime is called when exiting the time production.
	ExitTime(c *TimeContext)

	// ExitHour is called when exiting the hour production.
	ExitHour(c *HourContext)

	// ExitMinute is called when exiting the minute production.
	ExitMinute(c *MinuteContext)

	// ExitSecond is called when exiting the second production.
	ExitSecond(c *SecondContext)

	// ExitNano is called when exiting the nano production.
	ExitNano(c *NanoContext)

	// ExitTimezone is called when exiting the timezone production.
	ExitTimezone(c *TimezoneContext)

	// ExitTimezonenum is called when exiting the timezonenum production.
	ExitTimezonenum(c *TimezonenumContext)

	// ExitPri is called when exiting the pri production.
	ExitPri(c *PriContext)

	// ExitVersion is called when exiting the version production.
	ExitVersion(c *VersionContext)

	// ExitHostname is called when exiting the hostname production.
	ExitHostname(c *HostnameContext)

	// ExitAppname is called when exiting the appname production.
	ExitAppname(c *AppnameContext)

	// ExitMsgid is called when exiting the msgid production.
	ExitMsgid(c *MsgidContext)

	// ExitProcid is called when exiting the procid production.
	ExitProcid(c *ProcidContext)

	// ExitAllascii is called when exiting the allascii production.
	ExitAllascii(c *AllasciiContext)

	// ExitStructured is called when exiting the structured production.
	ExitStructured(c *StructuredContext)

	// ExitElement is called when exiting the element production.
	ExitElement(c *ElementContext)

	// ExitSid is called when exiting the sid production.
	ExitSid(c *SidContext)

	// ExitParam is called when exiting the param production.
	ExitParam(c *ParamContext)

	// ExitName is called when exiting the name production.
	ExitName(c *NameContext)

	// ExitSpecialname is called when exiting the specialname production.
	ExitSpecialname(c *SpecialnameContext)

	// ExitValue is called when exiting the value production.
	ExitValue(c *ValueContext)

	// ExitSpecialvalue is called when exiting the specialvalue production.
	ExitSpecialvalue(c *SpecialvalueContext)
}
