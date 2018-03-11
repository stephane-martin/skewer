// Code generated from grammars/rfc5424/RFC5424.g4 by ANTLR 4.7.1. DO NOT EDIT.

package rfc5424 // RFC5424
import "github.com/antlr/antlr4/runtime/Go/antlr"

// BaseRFC5424Listener is a complete listener for a parse tree produced by RFC5424Parser.
type BaseRFC5424Listener struct{}

var _ RFC5424Listener = &BaseRFC5424Listener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseRFC5424Listener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseRFC5424Listener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseRFC5424Listener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseRFC5424Listener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterFull is called when production full is entered.
func (s *BaseRFC5424Listener) EnterFull(ctx *FullContext) {}

// ExitFull is called when production full is exited.
func (s *BaseRFC5424Listener) ExitFull(ctx *FullContext) {}

// EnterHeadr is called when production headr is entered.
func (s *BaseRFC5424Listener) EnterHeadr(ctx *HeadrContext) {}

// ExitHeadr is called when production headr is exited.
func (s *BaseRFC5424Listener) ExitHeadr(ctx *HeadrContext) {}

// EnterMsg is called when production msg is entered.
func (s *BaseRFC5424Listener) EnterMsg(ctx *MsgContext) {}

// ExitMsg is called when production msg is exited.
func (s *BaseRFC5424Listener) ExitMsg(ctx *MsgContext) {}

// EnterAllchars is called when production allchars is entered.
func (s *BaseRFC5424Listener) EnterAllchars(ctx *AllcharsContext) {}

// ExitAllchars is called when production allchars is exited.
func (s *BaseRFC5424Listener) ExitAllchars(ctx *AllcharsContext) {}

// EnterTimestamp is called when production timestamp is entered.
func (s *BaseRFC5424Listener) EnterTimestamp(ctx *TimestampContext) {}

// ExitTimestamp is called when production timestamp is exited.
func (s *BaseRFC5424Listener) ExitTimestamp(ctx *TimestampContext) {}

// EnterDate is called when production date is entered.
func (s *BaseRFC5424Listener) EnterDate(ctx *DateContext) {}

// ExitDate is called when production date is exited.
func (s *BaseRFC5424Listener) ExitDate(ctx *DateContext) {}

// EnterYear is called when production year is entered.
func (s *BaseRFC5424Listener) EnterYear(ctx *YearContext) {}

// ExitYear is called when production year is exited.
func (s *BaseRFC5424Listener) ExitYear(ctx *YearContext) {}

// EnterMonth is called when production month is entered.
func (s *BaseRFC5424Listener) EnterMonth(ctx *MonthContext) {}

// ExitMonth is called when production month is exited.
func (s *BaseRFC5424Listener) ExitMonth(ctx *MonthContext) {}

// EnterDay is called when production day is entered.
func (s *BaseRFC5424Listener) EnterDay(ctx *DayContext) {}

// ExitDay is called when production day is exited.
func (s *BaseRFC5424Listener) ExitDay(ctx *DayContext) {}

// EnterTime is called when production time is entered.
func (s *BaseRFC5424Listener) EnterTime(ctx *TimeContext) {}

// ExitTime is called when production time is exited.
func (s *BaseRFC5424Listener) ExitTime(ctx *TimeContext) {}

// EnterHour is called when production hour is entered.
func (s *BaseRFC5424Listener) EnterHour(ctx *HourContext) {}

// ExitHour is called when production hour is exited.
func (s *BaseRFC5424Listener) ExitHour(ctx *HourContext) {}

// EnterMinute is called when production minute is entered.
func (s *BaseRFC5424Listener) EnterMinute(ctx *MinuteContext) {}

// ExitMinute is called when production minute is exited.
func (s *BaseRFC5424Listener) ExitMinute(ctx *MinuteContext) {}

// EnterSecond is called when production second is entered.
func (s *BaseRFC5424Listener) EnterSecond(ctx *SecondContext) {}

// ExitSecond is called when production second is exited.
func (s *BaseRFC5424Listener) ExitSecond(ctx *SecondContext) {}

// EnterNano is called when production nano is entered.
func (s *BaseRFC5424Listener) EnterNano(ctx *NanoContext) {}

// ExitNano is called when production nano is exited.
func (s *BaseRFC5424Listener) ExitNano(ctx *NanoContext) {}

// EnterTimezone is called when production timezone is entered.
func (s *BaseRFC5424Listener) EnterTimezone(ctx *TimezoneContext) {}

// ExitTimezone is called when production timezone is exited.
func (s *BaseRFC5424Listener) ExitTimezone(ctx *TimezoneContext) {}

// EnterTimezonenum is called when production timezonenum is entered.
func (s *BaseRFC5424Listener) EnterTimezonenum(ctx *TimezonenumContext) {}

// ExitTimezonenum is called when production timezonenum is exited.
func (s *BaseRFC5424Listener) ExitTimezonenum(ctx *TimezonenumContext) {}

// EnterPri is called when production pri is entered.
func (s *BaseRFC5424Listener) EnterPri(ctx *PriContext) {}

// ExitPri is called when production pri is exited.
func (s *BaseRFC5424Listener) ExitPri(ctx *PriContext) {}

// EnterVersion is called when production version is entered.
func (s *BaseRFC5424Listener) EnterVersion(ctx *VersionContext) {}

// ExitVersion is called when production version is exited.
func (s *BaseRFC5424Listener) ExitVersion(ctx *VersionContext) {}

// EnterHostname is called when production hostname is entered.
func (s *BaseRFC5424Listener) EnterHostname(ctx *HostnameContext) {}

// ExitHostname is called when production hostname is exited.
func (s *BaseRFC5424Listener) ExitHostname(ctx *HostnameContext) {}

// EnterAppname is called when production appname is entered.
func (s *BaseRFC5424Listener) EnterAppname(ctx *AppnameContext) {}

// ExitAppname is called when production appname is exited.
func (s *BaseRFC5424Listener) ExitAppname(ctx *AppnameContext) {}

// EnterMsgid is called when production msgid is entered.
func (s *BaseRFC5424Listener) EnterMsgid(ctx *MsgidContext) {}

// ExitMsgid is called when production msgid is exited.
func (s *BaseRFC5424Listener) ExitMsgid(ctx *MsgidContext) {}

// EnterProcid is called when production procid is entered.
func (s *BaseRFC5424Listener) EnterProcid(ctx *ProcidContext) {}

// ExitProcid is called when production procid is exited.
func (s *BaseRFC5424Listener) ExitProcid(ctx *ProcidContext) {}

// EnterAllascii is called when production allascii is entered.
func (s *BaseRFC5424Listener) EnterAllascii(ctx *AllasciiContext) {}

// ExitAllascii is called when production allascii is exited.
func (s *BaseRFC5424Listener) ExitAllascii(ctx *AllasciiContext) {}

// EnterStructured is called when production structured is entered.
func (s *BaseRFC5424Listener) EnterStructured(ctx *StructuredContext) {}

// ExitStructured is called when production structured is exited.
func (s *BaseRFC5424Listener) ExitStructured(ctx *StructuredContext) {}

// EnterElement is called when production element is entered.
func (s *BaseRFC5424Listener) EnterElement(ctx *ElementContext) {}

// ExitElement is called when production element is exited.
func (s *BaseRFC5424Listener) ExitElement(ctx *ElementContext) {}

// EnterSid is called when production sid is entered.
func (s *BaseRFC5424Listener) EnterSid(ctx *SidContext) {}

// ExitSid is called when production sid is exited.
func (s *BaseRFC5424Listener) ExitSid(ctx *SidContext) {}

// EnterParam is called when production param is entered.
func (s *BaseRFC5424Listener) EnterParam(ctx *ParamContext) {}

// ExitParam is called when production param is exited.
func (s *BaseRFC5424Listener) ExitParam(ctx *ParamContext) {}

// EnterName is called when production name is entered.
func (s *BaseRFC5424Listener) EnterName(ctx *NameContext) {}

// ExitName is called when production name is exited.
func (s *BaseRFC5424Listener) ExitName(ctx *NameContext) {}

// EnterSpecialname is called when production specialname is entered.
func (s *BaseRFC5424Listener) EnterSpecialname(ctx *SpecialnameContext) {}

// ExitSpecialname is called when production specialname is exited.
func (s *BaseRFC5424Listener) ExitSpecialname(ctx *SpecialnameContext) {}

// EnterValue is called when production value is entered.
func (s *BaseRFC5424Listener) EnterValue(ctx *ValueContext) {}

// ExitValue is called when production value is exited.
func (s *BaseRFC5424Listener) ExitValue(ctx *ValueContext) {}

// EnterSpecialvalue is called when production specialvalue is entered.
func (s *BaseRFC5424Listener) EnterSpecialvalue(ctx *SpecialvalueContext) {}

// ExitSpecialvalue is called when production specialvalue is exited.
func (s *BaseRFC5424Listener) ExitSpecialvalue(ctx *SpecialvalueContext) {}
