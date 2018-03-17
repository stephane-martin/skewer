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

	// EnterTimestamp is called when entering the timestamp production.
	EnterTimestamp(c *TimestampContext)

	// EnterDate is called when entering the date production.
	EnterDate(c *DateContext)

	// EnterTime is called when entering the time production.
	EnterTime(c *TimeContext)

	// EnterTimezone is called when entering the timezone production.
	EnterTimezone(c *TimezoneContext)

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

	// EnterValue is called when entering the value production.
	EnterValue(c *ValueContext)

	// ExitFull is called when exiting the full production.
	ExitFull(c *FullContext)

	// ExitHeadr is called when exiting the headr production.
	ExitHeadr(c *HeadrContext)

	// ExitMsg is called when exiting the msg production.
	ExitMsg(c *MsgContext)

	// ExitTimestamp is called when exiting the timestamp production.
	ExitTimestamp(c *TimestampContext)

	// ExitDate is called when exiting the date production.
	ExitDate(c *DateContext)

	// ExitTime is called when exiting the time production.
	ExitTime(c *TimeContext)

	// ExitTimezone is called when exiting the timezone production.
	ExitTimezone(c *TimezoneContext)

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

	// ExitValue is called when exiting the value production.
	ExitValue(c *ValueContext)
}
