// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

// Code generated from PowerShell v7.6.9 token.cs; DO NOT EDIT.

package psparse

// TokenKind values match PowerShell's TokenKind enum, so the reference parser's kinds compare by number and name.
const (
	Unknown                TokenKind = 0
	Variable               TokenKind = 1
	SplattedVariable       TokenKind = 2
	Parameter              TokenKind = 3
	Number                 TokenKind = 4
	Label                  TokenKind = 5
	Identifier             TokenKind = 6
	Generic                TokenKind = 7
	NewLine                TokenKind = 8
	LineContinuation       TokenKind = 9
	Comment                TokenKind = 10
	EndOfInput             TokenKind = 11
	StringLiteral          TokenKind = 12
	StringExpandable       TokenKind = 13
	HereStringLiteral      TokenKind = 14
	HereStringExpandable   TokenKind = 15
	LParen                 TokenKind = 16
	RParen                 TokenKind = 17
	LCurly                 TokenKind = 18
	RCurly                 TokenKind = 19
	LBracket               TokenKind = 20
	RBracket               TokenKind = 21
	AtParen                TokenKind = 22
	AtCurly                TokenKind = 23
	DollarParen            TokenKind = 24
	Semi                   TokenKind = 25
	AndAnd                 TokenKind = 26
	OrOr                   TokenKind = 27
	Ampersand              TokenKind = 28
	Pipe                   TokenKind = 29
	Comma                  TokenKind = 30
	MinusMinus             TokenKind = 31
	PlusPlus               TokenKind = 32
	DotDot                 TokenKind = 33
	ColonColon             TokenKind = 34
	Dot                    TokenKind = 35
	Exclaim                TokenKind = 36
	Multiply               TokenKind = 37
	Divide                 TokenKind = 38
	Rem                    TokenKind = 39
	Plus                   TokenKind = 40
	Minus                  TokenKind = 41
	Equals                 TokenKind = 42
	PlusEquals             TokenKind = 43
	MinusEquals            TokenKind = 44
	MultiplyEquals         TokenKind = 45
	DivideEquals           TokenKind = 46
	RemainderEquals        TokenKind = 47
	Redirection            TokenKind = 48
	RedirectInStd          TokenKind = 49
	Format                 TokenKind = 50
	Not                    TokenKind = 51
	Bnot                   TokenKind = 52
	And                    TokenKind = 53
	Or                     TokenKind = 54
	Xor                    TokenKind = 55
	Band                   TokenKind = 56
	Bor                    TokenKind = 57
	Bxor                   TokenKind = 58
	Join                   TokenKind = 59
	Ieq                    TokenKind = 60
	Ine                    TokenKind = 61
	Ige                    TokenKind = 62
	Igt                    TokenKind = 63
	Ilt                    TokenKind = 64
	Ile                    TokenKind = 65
	Ilike                  TokenKind = 66
	Inotlike               TokenKind = 67
	Imatch                 TokenKind = 68
	Inotmatch              TokenKind = 69
	Ireplace               TokenKind = 70
	Icontains              TokenKind = 71
	Inotcontains           TokenKind = 72
	Iin                    TokenKind = 73
	Inotin                 TokenKind = 74
	Isplit                 TokenKind = 75
	Ceq                    TokenKind = 76
	Cne                    TokenKind = 77
	Cge                    TokenKind = 78
	Cgt                    TokenKind = 79
	Clt                    TokenKind = 80
	Cle                    TokenKind = 81
	Clike                  TokenKind = 82
	Cnotlike               TokenKind = 83
	Cmatch                 TokenKind = 84
	Cnotmatch              TokenKind = 85
	Creplace               TokenKind = 86
	Ccontains              TokenKind = 87
	Cnotcontains           TokenKind = 88
	Cin                    TokenKind = 89
	Cnotin                 TokenKind = 90
	Csplit                 TokenKind = 91
	Is                     TokenKind = 92
	IsNot                  TokenKind = 93
	As                     TokenKind = 94
	PostfixPlusPlus        TokenKind = 95
	PostfixMinusMinus      TokenKind = 96
	Shl                    TokenKind = 97
	Shr                    TokenKind = 98
	Colon                  TokenKind = 99
	QuestionMark           TokenKind = 100
	QuestionQuestionEquals TokenKind = 101
	QuestionQuestion       TokenKind = 102
	QuestionDot            TokenKind = 103
	QuestionLBracket       TokenKind = 104
	Begin                  TokenKind = 119
	Break                  TokenKind = 120
	Catch                  TokenKind = 121
	Class                  TokenKind = 122
	Continue               TokenKind = 123
	Data                   TokenKind = 124
	Define                 TokenKind = 125
	Do                     TokenKind = 126
	Dynamicparam           TokenKind = 127
	Else                   TokenKind = 128
	ElseIf                 TokenKind = 129
	End                    TokenKind = 130
	Exit                   TokenKind = 131
	Filter                 TokenKind = 132
	Finally                TokenKind = 133
	For                    TokenKind = 134
	Foreach                TokenKind = 135
	From                   TokenKind = 136
	Function               TokenKind = 137
	If                     TokenKind = 138
	In                     TokenKind = 139
	Param                  TokenKind = 140
	Process                TokenKind = 141
	Return                 TokenKind = 142
	Switch                 TokenKind = 143
	Throw                  TokenKind = 144
	Trap                   TokenKind = 145
	Try                    TokenKind = 146
	Until                  TokenKind = 147
	Using                  TokenKind = 148
	Var                    TokenKind = 149
	While                  TokenKind = 150
	Workflow               TokenKind = 151
	Parallel               TokenKind = 152
	Sequence               TokenKind = 153
	InlineScript           TokenKind = 154
	Configuration          TokenKind = 155
	DynamicKeyword         TokenKind = 156
	Public                 TokenKind = 157
	Private                TokenKind = 158
	Static                 TokenKind = 159
	Interface              TokenKind = 160
	Enum                   TokenKind = 161
	Namespace              TokenKind = 162
	Module                 TokenKind = 163
	Type                   TokenKind = 164
	Assembly               TokenKind = 165
	Command                TokenKind = 166
	Hidden                 TokenKind = 167
	Base                   TokenKind = 168
	Default                TokenKind = 169
	Clean                  TokenKind = 170
)

const (
	FlagNone                             TokenFlags = 0x00000000
	FlagBinaryPrecedenceLogical          TokenFlags = 0x1
	FlagBinaryPrecedenceBitwise          TokenFlags = 0x2
	FlagBinaryPrecedenceComparison       TokenFlags = 0x5
	FlagBinaryPrecedenceCoalesce         TokenFlags = 0x7
	FlagBinaryPrecedenceAdd              TokenFlags = 0x9
	FlagBinaryPrecedenceMultiply         TokenFlags = 0xa
	FlagBinaryPrecedenceFormat           TokenFlags = 0xc
	FlagBinaryPrecedenceRange            TokenFlags = 0xd
	FlagBinaryPrecedenceMask             TokenFlags = 0x0000000f
	FlagKeyword                          TokenFlags = 0x00000010
	FlagScriptBlockBlockName             TokenFlags = 0x00000020
	FlagBinaryOperator                   TokenFlags = 0x00000100
	FlagUnaryOperator                    TokenFlags = 0x00000200
	FlagCaseSensitiveOperator            TokenFlags = 0x00000400
	FlagTernaryOperator                  TokenFlags = 0x00000800
	FlagSpecialOperator                  TokenFlags = 0x00001000
	FlagAssignmentOperator               TokenFlags = 0x00002000
	FlagParseModeInvariant               TokenFlags = 0x00008000
	FlagTokenInError                     TokenFlags = 0x00010000
	FlagDisallowedInRestrictedMode       TokenFlags = 0x00020000
	FlagPrefixOrPostfixOperator          TokenFlags = 0x00040000
	FlagCommandName                      TokenFlags = 0x00080000
	FlagMemberName                       TokenFlags = 0x00100000
	FlagTypeName                         TokenFlags = 0x00200000
	FlagAttributeName                    TokenFlags = 0x00400000
	FlagCanConstantFold                  TokenFlags = 0x00800000
	FlagStatementDoesntSupportAttributes TokenFlags = 0x01000000
)

// tokenKindNames is empty for the values PowerShell leaves unused.
var tokenKindNames = [...]string{
	"Unknown",                // Unknown
	"Variable",               // Variable
	"SplattedVariable",       // SplattedVariable
	"Parameter",              // Parameter
	"Number",                 // Number
	"Label",                  // Label
	"Identifier",             // Identifier
	"Generic",                // Generic
	"NewLine",                // Newline
	"LineContinuation",       // LineContinuation
	"Comment",                // Comment
	"EndOfInput",             // EndOfInput
	"StringLiteral",          // StringLiteral
	"StringExpandable",       // StringExpandable
	"HereStringLiteral",      // HereStringLiteral
	"HereStringExpandable",   // HereStringExpandable
	"LParen",                 // LParen
	"RParen",                 // RParen
	"LCurly",                 // LCurly
	"RCurly",                 // RCurly
	"LBracket",               // LBracket
	"RBracket",               // RBracket
	"AtParen",                // AtParen
	"AtCurly",                // AtCurly
	"DollarParen",            // DollarParen
	"Semi",                   // Semi
	"AndAnd",                 // AndAnd
	"OrOr",                   // OrOr
	"Ampersand",              // Ampersand
	"Pipe",                   // Pipe
	"Comma",                  // Comma
	"MinusMinus",             // MinusMinus
	"PlusPlus",               // PlusPlus
	"DotDot",                 // DotDot
	"ColonColon",             // ColonColon
	"Dot",                    // Dot
	"Exclaim",                // Exclaim
	"Multiply",               // Multiply
	"Divide",                 // Divide
	"Rem",                    // Rem
	"Plus",                   // Plus
	"Minus",                  // Minus
	"Equals",                 // Equals
	"PlusEquals",             // PlusEquals
	"MinusEquals",            // MinusEquals
	"MultiplyEquals",         // MultiplyEquals
	"DivideEquals",           // DivideEquals
	"RemainderEquals",        // RemainderEquals
	"Redirection",            // Redirection
	"RedirectInStd",          // RedirectInStd
	"Format",                 // Format
	"Not",                    // Not
	"Bnot",                   // Bnot
	"And",                    // And
	"Or",                     // Or
	"Xor",                    // Xor
	"Band",                   // Band
	"Bor",                    // Bor
	"Bxor",                   // Bxor
	"Join",                   // Join
	"Ieq",                    // Ieq
	"Ine",                    // Ine
	"Ige",                    // Ige
	"Igt",                    // Igt
	"Ilt",                    // Ilt
	"Ile",                    // Ile
	"Ilike",                  // Ilike
	"Inotlike",               // Inotlike
	"Imatch",                 // Imatch
	"Inotmatch",              // Inotmatch
	"Ireplace",               // Ireplace
	"Icontains",              // Icontains
	"Inotcontains",           // Inotcontains
	"Iin",                    // Iin
	"Inotin",                 // Inotin
	"Isplit",                 // Isplit
	"Ceq",                    // Ceq
	"Cne",                    // Cne
	"Cge",                    // Cge
	"Cgt",                    // Cgt
	"Clt",                    // Clt
	"Cle",                    // Cle
	"Clike",                  // Clike
	"Cnotlike",               // Cnotlike
	"Cmatch",                 // Cmatch
	"Cnotmatch",              // Cnotmatch
	"Creplace",               // Creplace
	"Ccontains",              // Ccontains
	"Cnotcontains",           // Cnotcontains
	"Cin",                    // Cin
	"Cnotin",                 // Cnotin
	"Csplit",                 // Csplit
	"Is",                     // Is
	"IsNot",                  // IsNot
	"As",                     // As
	"PostfixPlusPlus",        // PostFixPlusPlus
	"PostfixMinusMinus",      // PostFixMinusMinus
	"Shl",                    // Shl
	"Shr",                    // Shr
	"Colon",                  // Colon
	"QuestionMark",           // QuestionMark
	"QuestionQuestionEquals", // QuestionQuestionEquals
	"QuestionQuestion",       // QuestionQuestion
	"QuestionDot",            // QuestionDot
	"QuestionLBracket",       // QuestionLBracket
	"",                       // Reserved slot 7
	"",                       // Reserved slot 8
	"",                       // Reserved slot 9
	"",                       // Reserved slot 10
	"",                       // Reserved slot 11
	"",                       // Reserved slot 12
	"",                       // Reserved slot 13
	"",                       // Reserved slot 14
	"",                       // Reserved slot 15
	"",                       // Reserved slot 16
	"",                       // Reserved slot 17
	"",                       // Reserved slot 18
	"",                       // Reserved slot 19
	"",                       // Reserved slot 20
	"Begin",                  // Begin
	"Break",                  // Break
	"Catch",                  // Catch
	"Class",                  // Class
	"Continue",               // Continue
	"Data",                   // Data
	"Define",                 // Define
	"Do",                     // Do
	"Dynamicparam",           // Dynamicparam
	"Else",                   // Else
	"ElseIf",                 // ElseIf
	"End",                    // End
	"Exit",                   // Exit
	"Filter",                 // Filter
	"Finally",                // Finally
	"For",                    // For
	"Foreach",                // Foreach
	"From",                   // From
	"Function",               // Function
	"If",                     // If
	"In",                     // In
	"Param",                  // Param
	"Process",                // Process
	"Return",                 // Return
	"Switch",                 // Switch
	"Throw",                  // Throw
	"Trap",                   // Trap
	"Try",                    // Try
	"Until",                  // Until
	"Using",                  // Using
	"Var",                    // Var
	"While",                  // While
	"Workflow",               // Workflow
	"Parallel",               // Parallel
	"Sequence",               // Sequence
	"InlineScript",           // InlineScript
	"Configuration",          // Configuration
	"DynamicKeyword",         // <dynamic keyword>
	"Public",                 // Public
	"Private",                // Private
	"Static",                 // Static
	"Interface",              // Interface
	"Enum",                   // Enum
	"Namespace",              // Namespace
	"Module",                 // Module
	"Type",                   // Type
	"Assembly",               // Assembly
	"Command",                // Command
	"Hidden",                 // Hidden
	"Base",                   // Base
	"Default",                // Default
	"Clean",                  // Clean
}

var staticTokenFlags = [...]TokenFlags{
	/* Unknown */ FlagNone,
	/* Variable */ FlagNone,
	/* SplattedVariable */ FlagNone,
	/* Parameter */ FlagNone,
	/* Number */ FlagNone,
	/* Label */ FlagNone,
	/* Identifier */ FlagNone,
	/* Generic */ FlagNone,
	/* Newline */ FlagParseModeInvariant,
	/* LineContinuation */ FlagParseModeInvariant,
	/* Comment */ FlagParseModeInvariant,
	/* EndOfInput */ FlagParseModeInvariant,
	/* StringLiteral */ FlagParseModeInvariant,
	/* StringExpandable */ FlagParseModeInvariant,
	/* HereStringLiteral */ FlagParseModeInvariant,
	/* HereStringExpandable */ FlagParseModeInvariant,
	/* LParen */ FlagParseModeInvariant,
	/* RParen */ FlagParseModeInvariant,
	/* LCurly */ FlagParseModeInvariant,
	/* RCurly */ FlagParseModeInvariant,
	/* LBracket */ FlagNone,
	/* RBracket */ FlagParseModeInvariant,
	/* AtParen */ FlagParseModeInvariant,
	/* AtCurly */ FlagParseModeInvariant,
	/* DollarParen */ FlagParseModeInvariant,
	/* Semi */ FlagParseModeInvariant,
	/* AndAnd */ FlagParseModeInvariant,
	/* OrOr */ FlagParseModeInvariant,
	/* Ampersand */ FlagSpecialOperator | FlagParseModeInvariant,
	/* Pipe */ FlagSpecialOperator | FlagParseModeInvariant,
	/* Comma */ FlagUnaryOperator | FlagParseModeInvariant,
	/* MinusMinus */ FlagUnaryOperator | FlagPrefixOrPostfixOperator | FlagDisallowedInRestrictedMode,
	/* PlusPlus */ FlagUnaryOperator | FlagPrefixOrPostfixOperator | FlagDisallowedInRestrictedMode,
	/* DotDot */ FlagBinaryOperator | FlagBinaryPrecedenceRange | FlagDisallowedInRestrictedMode,
	/* ColonColon */ FlagSpecialOperator | FlagDisallowedInRestrictedMode,
	/* Dot */ FlagSpecialOperator | FlagDisallowedInRestrictedMode,
	/* Exclaim */ FlagUnaryOperator | FlagCanConstantFold,
	/* Multiply */ FlagBinaryOperator | FlagBinaryPrecedenceMultiply | FlagCanConstantFold,
	/* Divide */ FlagBinaryOperator | FlagBinaryPrecedenceMultiply | FlagCanConstantFold,
	/* Rem */ FlagBinaryOperator | FlagBinaryPrecedenceMultiply | FlagCanConstantFold,
	/* Plus */ FlagBinaryOperator | FlagBinaryPrecedenceAdd | FlagUnaryOperator | FlagCanConstantFold,
	/* Minus */ FlagBinaryOperator | FlagBinaryPrecedenceAdd | FlagUnaryOperator | FlagCanConstantFold,
	/* Equals */ FlagAssignmentOperator,
	/* PlusEquals */ FlagAssignmentOperator,
	/* MinusEquals */ FlagAssignmentOperator,
	/* MultiplyEquals */ FlagAssignmentOperator,
	/* DivideEquals */ FlagAssignmentOperator,
	/* RemainderEquals */ FlagAssignmentOperator,
	/* Redirection */ FlagDisallowedInRestrictedMode,
	/* RedirectInStd */ FlagParseModeInvariant | FlagDisallowedInRestrictedMode,
	/* Format */ FlagBinaryOperator | FlagBinaryPrecedenceFormat | FlagDisallowedInRestrictedMode,
	/* Not */ FlagUnaryOperator | FlagCanConstantFold,
	/* Bnot */ FlagUnaryOperator | FlagCanConstantFold,
	/* And */ FlagBinaryOperator | FlagBinaryPrecedenceLogical | FlagCanConstantFold,
	/* Or */ FlagBinaryOperator | FlagBinaryPrecedenceLogical | FlagCanConstantFold,
	/* Xor */ FlagBinaryOperator | FlagBinaryPrecedenceLogical | FlagCanConstantFold,
	/* Band */ FlagBinaryOperator | FlagBinaryPrecedenceBitwise | FlagCanConstantFold,
	/* Bor */ FlagBinaryOperator | FlagBinaryPrecedenceBitwise | FlagCanConstantFold,
	/* Bxor */ FlagBinaryOperator | FlagBinaryPrecedenceBitwise | FlagCanConstantFold,
	/* Join */ FlagBinaryOperator | FlagUnaryOperator | FlagBinaryPrecedenceComparison | FlagDisallowedInRestrictedMode,
	/* Ieq */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Ine */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Ige */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Igt */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Ilt */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Ile */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Ilike */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Inotlike */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Imatch */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagDisallowedInRestrictedMode,
	/* Inotmatch */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagDisallowedInRestrictedMode,
	/* Ireplace */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagDisallowedInRestrictedMode,
	/* Icontains */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Inotcontains */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Iin */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Inotin */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* Isplit */ FlagBinaryOperator | FlagUnaryOperator | FlagBinaryPrecedenceComparison | FlagDisallowedInRestrictedMode,
	/* Ceq */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cne */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cge */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cgt */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Clt */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cle */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Clike */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cnotlike */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cmatch */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator | FlagDisallowedInRestrictedMode,
	/* Cnotmatch */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator | FlagDisallowedInRestrictedMode,
	/* Creplace */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator | FlagDisallowedInRestrictedMode,
	/* Ccontains */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cnotcontains */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cin */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Cnotin */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator,
	/* Csplit */ FlagBinaryOperator | FlagUnaryOperator | FlagBinaryPrecedenceComparison | FlagCaseSensitiveOperator | FlagDisallowedInRestrictedMode,
	/* Is */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* IsNot */ FlagBinaryOperator | FlagBinaryPrecedenceComparison,
	/* As */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagDisallowedInRestrictedMode,
	/* PostFixPlusPlus */ FlagUnaryOperator | FlagPrefixOrPostfixOperator | FlagDisallowedInRestrictedMode,
	/* PostFixMinusMinus */ FlagUnaryOperator | FlagPrefixOrPostfixOperator | FlagDisallowedInRestrictedMode,
	/* Shl */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCanConstantFold,
	/* Shr */ FlagBinaryOperator | FlagBinaryPrecedenceComparison | FlagCanConstantFold,
	/* Colon */ FlagSpecialOperator | FlagDisallowedInRestrictedMode,
	/* QuestionMark */ FlagTernaryOperator | FlagDisallowedInRestrictedMode,
	/* QuestionQuestionEquals */ FlagAssignmentOperator,
	/* QuestionQuestion */ FlagBinaryOperator | FlagBinaryPrecedenceCoalesce,
	/* QuestionDot */ FlagSpecialOperator | FlagDisallowedInRestrictedMode,
	/* QuestionLBracket */ FlagNone,
	/* Reserved slot 7 */ FlagNone,
	/* Reserved slot 8 */ FlagNone,
	/* Reserved slot 9 */ FlagNone,
	/* Reserved slot 10 */ FlagNone,
	/* Reserved slot 11 */ FlagNone,
	/* Reserved slot 12 */ FlagNone,
	/* Reserved slot 13 */ FlagNone,
	/* Reserved slot 14 */ FlagNone,
	/* Reserved slot 15 */ FlagNone,
	/* Reserved slot 16 */ FlagNone,
	/* Reserved slot 17 */ FlagNone,
	/* Reserved slot 18 */ FlagNone,
	/* Reserved slot 19 */ FlagNone,
	/* Reserved slot 20 */ FlagNone,
	/* Begin */ FlagKeyword | FlagScriptBlockBlockName,
	/* Break */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Catch */ FlagKeyword,
	/* Class */ FlagKeyword,
	/* Continue */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Data */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Define */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Do */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Dynamicparam */ FlagKeyword | FlagScriptBlockBlockName,
	/* Else */ FlagKeyword,
	/* ElseIf */ FlagKeyword,
	/* End */ FlagKeyword | FlagScriptBlockBlockName,
	/* Exit */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Filter */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Finally */ FlagKeyword,
	/* For */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Foreach */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* From */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Function */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* If */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* In */ FlagKeyword,
	/* Param */ FlagKeyword,
	/* Process */ FlagKeyword | FlagScriptBlockBlockName,
	/* Return */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Switch */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Throw */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Trap */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Try */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Until */ FlagKeyword,
	/* Using */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Var */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* While */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Workflow */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Parallel */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Sequence */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* InlineScript */ FlagKeyword | FlagStatementDoesntSupportAttributes,
	/* Configuration */ FlagKeyword,
	/* <dynamic keyword> */ FlagKeyword,
	/* Public */ FlagKeyword,
	/* Private */ FlagKeyword,
	/* Static */ FlagKeyword,
	/* Interface */ FlagKeyword,
	/* Enum */ FlagKeyword,
	/* Namespace */ FlagKeyword,
	/* Module */ FlagKeyword,
	/* Type */ FlagKeyword,
	/* Assembly */ FlagKeyword,
	/* Command */ FlagKeyword,
	/* Hidden */ FlagKeyword,
	/* Base */ FlagKeyword,
	/* Default */ FlagKeyword,
	/* Clean */ FlagKeyword | FlagScriptBlockBlockName,
}

var tokenText = [...]string{
	/* Unknown */ "unknown",
	/* Variable */ "var",
	/* SplattedVariable */ "@var",
	/* Parameter */ "param",
	/* Number */ "number",
	/* Label */ "label",
	/* Identifier */ "ident",
	/* Generic */ "generic",
	/* Newline */ "newline",
	/* LineContinuation */ "line continuation",
	/* Comment */ "comment",
	/* EndOfInput */ "eof",
	/* StringLiteral */ "sqstr",
	/* StringExpandable */ "dqstr",
	/* HereStringLiteral */ "sq here string",
	/* HereStringExpandable */ "dq here string",
	/* LParen */ "(",
	/* RParen */ ")",
	/* LCurly */ "{",
	/* RCurly */ "}",
	/* LBracket */ "[",
	/* RBracket */ "]",
	/* AtParen */ "@(",
	/* AtCurly */ "@{",
	/* DollarParen */ "$(",
	/* Semi */ ";",
	/* AndAnd */ "&&",
	/* OrOr */ "||",
	/* Ampersand */ "&",
	/* Pipe */ "|",
	/* Comma */ ",",
	/* MinusMinus */ "--",
	/* PlusPlus */ "++",
	/* DotDot */ "..",
	/* ColonColon */ "::",
	/* Dot */ ".",
	/* Exclaim */ "!",
	/* Multiply */ "*",
	/* Divide */ "/",
	/* Rem */ "%",
	/* Plus */ "+",
	/* Minus */ "-",
	/* Equals */ "=",
	/* PlusEquals */ "+=",
	/* MinusEquals */ "-=",
	/* MultiplyEquals */ "*=",
	/* DivideEquals */ "/=",
	/* RemainderEquals */ "%=",
	/* Redirection */ "redirection",
	/* RedirectInStd */ "<",
	/* Format */ "-f",
	/* Not */ "-not",
	/* Bnot */ "-bnot",
	/* And */ "-and",
	/* Or */ "-or",
	/* Xor */ "-xor",
	/* Band */ "-band",
	/* Bor */ "-bor",
	/* Bxor */ "-bxor",
	/* Join */ "-join",
	/* Ieq */ "-eq",
	/* Ine */ "-ne",
	/* Ige */ "-ge",
	/* Igt */ "-gt",
	/* Ilt */ "-lt",
	/* Ile */ "-le",
	/* Ilike */ "-ilike",
	/* Inotlike */ "-inotlike",
	/* Imatch */ "-imatch",
	/* Inotmatch */ "-inotmatch",
	/* Ireplace */ "-ireplace",
	/* Icontains */ "-icontains",
	/* Inotcontains */ "-inotcontains",
	/* Iin */ "-iin",
	/* Inotin */ "-inotin",
	/* Isplit */ "-isplit",
	/* Ceq */ "-ceq",
	/* Cne */ "-cne",
	/* Cge */ "-cge",
	/* Cgt */ "-cgt",
	/* Clt */ "-clt",
	/* Cle */ "-cle",
	/* Clike */ "-clike",
	/* Cnotlike */ "-cnotlike",
	/* Cmatch */ "-cmatch",
	/* Cnotmatch */ "-cnotmatch",
	/* Creplace */ "-creplace",
	/* Ccontains */ "-ccontains",
	/* Cnotcontains */ "-cnotcontains",
	/* Cin */ "-cin",
	/* Cnotin */ "-cnotin",
	/* Csplit */ "-csplit",
	/* Is */ "-is",
	/* IsNot */ "-isnot",
	/* As */ "-as",
	/* PostFixPlusPlus */ "++",
	/* PostFixMinusMinus */ "--",
	/* Shl */ "-shl",
	/* Shr */ "-shr",
	/* Colon */ ":",
	/* QuestionMark */ "?",
	/* QuestionQuestionEquals */ "??=",
	/* QuestionQuestion */ "??",
	/* QuestionDot */ "?.",
	/* QuestionLBracket */ "?[",
	/* Reserved slot 7 */ "",
	/* Reserved slot 8 */ "",
	/* Reserved slot 9 */ "",
	/* Reserved slot 10 */ "",
	/* Reserved slot 11 */ "",
	/* Reserved slot 12 */ "",
	/* Reserved slot 13 */ "",
	/* Reserved slot 14 */ "",
	/* Reserved slot 15 */ "",
	/* Reserved slot 16 */ "",
	/* Reserved slot 17 */ "",
	/* Reserved slot 18 */ "",
	/* Reserved slot 19 */ "",
	/* Reserved slot 20 */ "",
	/* Begin */ "begin",
	/* Break */ "break",
	/* Catch */ "catch",
	/* Class */ "class",
	/* Continue */ "continue",
	/* Data */ "data",
	/* Define */ "define",
	/* Do */ "do",
	/* Dynamicparam */ "dynamicparam",
	/* Else */ "else",
	/* ElseIf */ "elseif",
	/* End */ "end",
	/* Exit */ "exit",
	/* Filter */ "filter",
	/* Finally */ "finally",
	/* For */ "for",
	/* Foreach */ "foreach",
	/* From */ "from",
	/* Function */ "function",
	/* If */ "if",
	/* In */ "in",
	/* Param */ "param",
	/* Process */ "process",
	/* Return */ "return",
	/* Switch */ "switch",
	/* Throw */ "throw",
	/* Trap */ "trap",
	/* Try */ "try",
	/* Until */ "until",
	/* Using */ "using",
	/* Var */ "var",
	/* While */ "while",
	/* Workflow */ "workflow",
	/* Parallel */ "parallel",
	/* Sequence */ "sequence",
	/* InlineScript */ "inlinescript",
	/* Configuration */ "configuration",
	/* <dynamic keyword> */ "<dynamic keyword>",
	/* Public */ "public",
	/* Private */ "private",
	/* Static */ "static",
	/* Interface */ "interface",
	/* Enum */ "enum",
	/* Namespace */ "namespace",
	/* Module */ "module",
	/* Type */ "type",
	/* Assembly */ "assembly",
	/* Command */ "command",
	/* Hidden */ "hidden",
	/* Base */ "base",
	/* Default */ "default",
	/* Clean */ "clean",
}
