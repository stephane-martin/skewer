grammar RFC5424;

full:         headr SP+ structured (SP+ msg)? EOF;
headr:        pri version SP+ timestamp SP+ hostname SP+ appname SP+ procid SP+ msgid;

msg:          allchars*?;
allchars:     ASCII | VALUECHAR | specialvalue | ANTISLASH | CLOSESQUARE | QUOTE | ANYCHAR;

timestamp:    date ASCII time timezone;
date:         year HYPHEN month HYPHEN day;
year:         DIGIT DIGIT DIGIT DIGIT;
month:        DIGIT DIGIT;
day:          DIGIT DIGIT;
time:         hour COLON minute COLON second (DOT nano)?;
hour:         DIGIT DIGIT;
minute:       DIGIT DIGIT;
second:       DIGIT DIGIT;
nano:         DIGIT+;
timezone:     ASCII | timezonenum;
timezonenum:  (PLUS | HYPHEN) hour COLON minute;

pri:          OPENBRACKET DIGIT+ CLOSEBRACKET;
version:      DIGIT;

hostname:     HYPHEN | (allascii allascii+);
appname:      HYPHEN | (allascii allascii+);
msgid:        HYPHEN | (allascii allascii+);
procid:       HYPHEN | (allascii allascii+);

allascii:     ASCII | specialname | EQUAL | CLOSESQUARE | QUOTE;

structured:   HYPHEN | (element+);
element:      OPENSQUARE sid (SP param)* CLOSESQUARE;
sid:          name;
param:        name EQUAL QUOTE value QUOTE;

name:         (ASCII | specialname)+;
specialname:  DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | OPENSQUARE | ANTISLASH;

value:        (ASCII | VALUECHAR | specialvalue | (ANTISLASH ANTISLASH) | (ANTISLASH CLOSESQUARE) | (ANTISLASH QUOTE))*;
specialvalue: DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | SP | EQUAL | OPENSQUARE;

DIGIT:        [0-9];
DOT:          '.';
OPENBRACKET:  '<';
CLOSEBRACKET: '>';
PLUS:         '+';
HYPHEN:       '-';
COLON:        ':';
SP:           ' ';
EQUAL:        '=';
QUOTE:        '"';
OPENSQUARE:   '[';
CLOSESQUARE:  ']';
ANTISLASH:    '\\';
ASCII:        [!#$%&'()*,/;?@A-Z^_`a-z{|}~];
VALUECHAR:    ~[ ="\\[\]<>\-:.+];    
ANYCHAR:      .;

