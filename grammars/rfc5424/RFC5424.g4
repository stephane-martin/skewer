grammar RFC5424;

full:         headr SP+ structured (SP+ msg)? EOF;
headr:        OPENBRACKET pri CLOSEBRACKET version SP+ timestamp SP+ hostname SP+ appname SP+ procid SP+ msgid;
msg:          .*?;

timestamp:    date ASCII time timezone;
date:         DIGIT DIGIT DIGIT DIGIT HYPHEN DIGIT DIGIT HYPHEN DIGIT DIGIT;
time:         DIGIT DIGIT COLON DIGIT DIGIT COLON DIGIT DIGIT (DOT DIGIT+)?;
timezone:     ASCII | ((PLUS | HYPHEN) DIGIT DIGIT COLON DIGIT DIGIT);

pri:          DIGIT+;
version:      DIGIT+;

hostname:     (ASCII | DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | OPENSQUARE | ANTISLASH | EQUAL | CLOSESQUARE | QUOTE)+;
appname:      (ASCII | DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | OPENSQUARE | ANTISLASH | EQUAL | CLOSESQUARE | QUOTE)+;
msgid:        (ASCII | DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | OPENSQUARE | ANTISLASH | EQUAL | CLOSESQUARE | QUOTE)+;
procid:       (ASCII | DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | OPENSQUARE | ANTISLASH | EQUAL | CLOSESQUARE | QUOTE)+;

structured:   HYPHEN | (element+);
element:      OPENSQUARE sid (SP param)* CLOSESQUARE;
sid:          name;
param:        name EQUAL QUOTE value QUOTE;
name:         (ASCII | DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | OPENSQUARE | ANTISLASH)+;
value:        (ASCII | VALUECHAR | DIGIT | DOT | OPENBRACKET | CLOSEBRACKET | PLUS | HYPHEN | COLON | SP | EQUAL | OPENSQUARE | (ANTISLASH ANTISLASH) | (ANTISLASH CLOSESQUARE) | (ANTISLASH QUOTE))*;

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

