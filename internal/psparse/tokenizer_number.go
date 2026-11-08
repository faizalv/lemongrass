// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import (
	"math"
	"math/big"
	"strconv"
)

// numberFormat is NumberFormat.
type numberFormat int

const (
	formatDecimal numberFormat = iota
	formatHex
	formatBinary
)

// numberSuffix is NumberSuffixFlags.
type numberSuffix int

const (
	suffixNone          numberSuffix = 0x0
	suffixUnsigned      numberSuffix = 0x1
	suffixSignedByte    numberSuffix = 0x2
	suffixUnsignedByte  numberSuffix = 0x3
	suffixShort         numberSuffix = 0x4
	suffixUnsignedShort numberSuffix = 0x5
	suffixLong          numberSuffix = 0x8
	suffixUnsignedLong  numberSuffix = 0x9
	suffixDecimal       numberSuffix = 0x10
	suffixBigInteger    numberSuffix = 0x20
)

// scanNumber is ScanNumber. Text that is not a number (77z) or does not fit its type is a generic token outside
// expression mode and BadNumericConstant inside it.
func (t *tokenizer) scanNumber(first uint16) *Token {
	strNum, format, suffix, real, multiplier, ok := t.scanNumberHelperFull(first)
	if !ok {
		t.currentIndex = t.tokenStart
		return t.scanGenericToken(nil)
	}
	value, ok := tryGetNumberValue(strNum, format, suffix, real, multiplier)
	if !ok {
		if !t.inExpressionMode() {
			t.currentIndex = t.tokenStart
			return t.scanGenericToken(nil)
		}
		t.parser.reportError(t.newScriptExtent(t.tokenStart, t.currentIndex), "BadNumericConstant",
			utf16ToString(t.script[t.tokenStart:min(t.currentIndex, len(t.script))]))
	}
	return t.newNumberToken(value)
}

// scanNumberHelper reports only whether a number follows, as NextToken's '!' case uses ScanNumberHelper.
func (t *tokenizer) scanNumberHelper(first uint16) (string, numberFormat, numberSuffix, bool, bool) {
	s, f, sfx, real, _, ok := t.scanNumberHelperFull(first)
	return s, f, sfx, real, ok
}

func (t *tokenizer) scanDecimalDigits(sb *[]uint16) int {
	n := 0
	for c := t.peekChar(); isDecimalDigit(c); c = t.peekChar() {
		n++
		t.skipChar()
		*sb = append(*sb, c)
	}
	return n
}

func (t *tokenizer) scanHexDigits(sb *[]uint16) {
	for c := t.peekChar(); isHexDigit(c); c = t.peekChar() {
		t.skipChar()
		*sb = append(*sb, c)
	}
}

func (t *tokenizer) scanBinaryDigits(sb *[]uint16) {
	for c := t.peekChar(); isBinaryDigit(c); c = t.peekChar() {
		t.skipChar()
		*sb = append(*sb, c)
	}
}

// scanExponent is ScanExponent.
func (t *tokenizer) scanExponent(sb *[]uint16, signIndex *int, notNumber *bool) {
	if c := t.peekChar(); c == '+' || isDash(c) {
		t.skipChar()
		*signIndex = len(*sb)
		*sb = append(*sb, c)
	}
	if t.scanDecimalDigits(sb) == 0 {
		*notNumber = true
	}
}

// scanNumberAfterDot is ScanNumberAfterDot.
func (t *tokenizer) scanNumberAfterDot(sb *[]uint16, signIndex *int, notNumber *bool) {
	t.scanDecimalDigits(sb)
	if c := t.peekChar(); c == 'e' || c == 'E' {
		t.skipChar()
		*sb = append(*sb, c)
		t.scanExponent(sb, signIndex, notNumber)
	}
}

// scanNumberHelperFull is ScanNumberHelper: the number's text with dashes normalized, its format, suffix,
// whether it is real, and its multiplier; ok is false when the text is not a number.
func (t *tokenizer) scanNumberHelperFull(first uint16) (string, numberFormat, numberSuffix, bool, int64, bool) {
	format := formatDecimal
	suffix := suffixNone
	real := false
	var multiplier int64 = 1
	notNumber := false
	signIndex := -1
	var sb []uint16
	var c uint16

	if isDash(first) || first == '+' {
		sb = append(sb, first)
		first = t.getChar()
	}
	if first == '.' {
		sb = append(sb, '.')
		t.scanNumberAfterDot(&sb, &signIndex, &notNumber)
		real = true
	} else {
		c = t.peekChar()
		if first == '0' && (c == 'x' || c == 'X' || c == 'b' || c == 'B') {
			t.skipChar()
			switch c {
			case 'x', 'X':
				// The leading 0 keeps the sign bit out of the way unless the length says it is meant.
				sb = append(sb, '0')
				t.scanHexDigits(&sb)
				format = formatHex
			case 'b', 'B':
				t.scanBinaryDigits(&sb)
				if len(sb) == 0 {
					notNumber = true
				}
				format = formatBinary
			}
		} else {
			sb = append(sb, first)
			t.scanDecimalDigits(&sb)
			switch c = t.peekChar(); c {
			case '.':
				t.skipChar()
				if t.peekChar() == '.' {
					// The range operator: stop before it.
					t.ungetChar()
				} else {
					sb = append(sb, c)
					real = true
					t.scanNumberAfterDot(&sb, &signIndex, &notNumber)
				}
			case 'E', 'e':
				t.skipChar()
				sb = append(sb, c)
				real = true
				t.scanExponent(&sb, &signIndex, &notNumber)
			}
		}
	}

	c = t.peekChar()
	if isTypeSuffix(c) {
		t.skipChar()
		switch c {
		case 'u', 'U':
			suffix |= suffixUnsigned
		case 's', 'S':
			suffix |= suffixShort
		case 'l', 'L':
			suffix |= suffixLong
		case 'd', 'D':
			suffix |= suffixDecimal
		case 'y', 'Y':
			suffix |= suffixSignedByte
		case 'n', 'N':
			suffix |= suffixBigInteger
		default:
			notNumber = true
		}
		c = t.peekChar()
		if isTypeSuffix(c) {
			t.skipChar()
			if suffix == suffixUnsigned {
				switch c {
				case 'l', 'L':
					suffix |= suffixLong
				case 's', 'S':
					suffix |= suffixShort
				case 'y', 'Y':
					suffix |= suffixSignedByte
				default:
					notNumber = true
				}
			} else {
				notNumber = true
			}
			c = t.peekChar()
		}
	}

	if isMultiplierStart(c) {
		t.skipChar()
		switch c {
		case 'k', 'K':
			multiplier = 1024
		case 'm', 'M':
			multiplier = 1024 * 1024
		case 'g', 'G':
			multiplier = 1024 * 1024 * 1024
		case 't', 'T':
			multiplier = 1024 * 1024 * 1024 * 1024
		case 'p', 'P':
			multiplier = 1024 * 1024 * 1024 * 1024 * 1024
		}
		if c1 := t.peekChar(); c1 == 'b' || c1 == 'B' {
			t.skipChar()
			c = t.peekChar()
		} else {
			notNumber = true
		}
	}

	if !forceStartNewToken(c) {
		if !t.inExpressionMode() || !forceStartNewTokenAfterNumber(c, t.ForceEndNumberOnTernaryOpChars) {
			notNumber = true
		}
	}
	if notNumber {
		return "", format, suffix, real, multiplier, false
	}
	if signIndex != -1 && sb[signIndex] != '-' && isDash(sb[signIndex]) {
		sb[signIndex] = '-'
	}
	if sb[0] != '-' && isDash(sb[0]) {
		sb[0] = '-'
	}
	return utf16ToString(sb), format, suffix, real, multiplier, true
}

var (
	bigInt32Min  = big.NewInt(math.MinInt32)
	bigInt32Max  = big.NewInt(math.MaxInt32)
	bigInt64Min  = big.NewInt(math.MinInt64)
	bigInt64Max  = big.NewInt(math.MaxInt64)
	bigUint32Max = new(big.Int).SetUint64(math.MaxUint32)
	bigUint64Max = new(big.Int).SetUint64(math.MaxUint64)
	// bigDecimalMax is decimal.MaxValue, 2^96 - 1.
	bigDecimalMax = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 96), big.NewInt(1))
)

func inRange(v *big.Int, lo, hi int64) bool {
	return v.Cmp(big.NewInt(lo)) >= 0 && v.Cmp(big.NewInt(hi)) <= 0
}

// castBig is the Utils.TryCast overload for suffix, picking the .NET type C# casts to.
func castBig(v *big.Int, suffix numberSuffix) (any, bool) {
	switch suffix {
	case suffixSignedByte:
		if inRange(v, math.MinInt8, math.MaxInt8) {
			return int8(v.Int64()), true
		}
	case suffixUnsignedByte:
		if inRange(v, 0, math.MaxUint8) {
			return uint8(v.Int64()), true
		}
	case suffixShort:
		if inRange(v, math.MinInt16, math.MaxInt16) {
			return int16(v.Int64()), true
		}
	case suffixLong:
		if v.Cmp(bigInt64Min) >= 0 && v.Cmp(bigInt64Max) <= 0 {
			return v.Int64(), true
		}
	case suffixUnsignedShort:
		if inRange(v, 0, math.MaxUint16) {
			return uint16(v.Int64()), true
		}
	case suffixUnsigned:
		if v.Sign() >= 0 && v.Cmp(bigUint32Max) <= 0 {
			return uint32(v.Uint64()), true
		}
		if v.Sign() >= 0 && v.Cmp(bigUint64Max) <= 0 {
			return v.Uint64(), true
		}
	case suffixUnsignedLong:
		if v.Sign() >= 0 && v.Cmp(bigUint64Max) <= 0 {
			return v.Uint64(), true
		}
	case suffixDecimal:
		if new(big.Int).Abs(v).Cmp(bigDecimalMax) <= 0 {
			return Decimal{Unscaled: new(big.Int).Set(v), Scale: 0}, true
		}
	case suffixBigInteger:
		return new(big.Int).Set(v), true
	}
	return nil, false
}

// tryGetNumberValue is TryGetNumberValue: the value PowerShell gives the literal, as the .NET type it picks.
func tryGetNumberValue(strNum string, format numberFormat, suffix numberSuffix, real bool, multiplier int64) (any, bool) {
	if strNum == "" {
		return nil, false
	}
	if real {
		if suffix == suffixDecimal {
			d, ok := parseDecimal(strNum)
			if !ok {
				return nil, false
			}
			return decimalMultiply(d, multiplier)
		}
		f, err := strconv.ParseFloat(strNum, 64)
		if err != nil && !math.IsInf(f, 0) {
			return nil, false
		}
		if f == 0 && strNum[0] == '-' {
			f = math.Copysign(0, -1)
		}
		f *= float64(multiplier)
		if suffix == suffixNone {
			return f, true
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			// new BigInteger(double) throws on infinity, and the catch makes the literal no number.
			return nil, false
		}
		bf := new(big.Float).SetFloat64(math.Trunc(f))
		bi, _ := bf.Int(nil)
		return castBig(bi, suffix)
	}

	var bigValue *big.Int
	digits := strNum
	switch format {
	case formatHex:
		if !isHexDigit(uint16(digits[0])) {
			if digits[0] == '-' {
				multiplier = -multiplier
			}
			digits = digits[1:]
		}
		if suffix&suffixUnsigned == 0 {
			expected := 8
			switch suffix {
			case suffixSignedByte:
				expected = 2
			case suffixShort:
				expected = 4
			case suffixLong:
				expected = 16
			default:
				if len(digits) >= 16 {
					expected = 16
				}
			}
			if len(digits) == expected+1 {
				digits = digits[1:]
			}
		}
		v, ok := parseHexTwosComplement(digits)
		if !ok {
			return nil, false
		}
		bigValue = v
		if len(digits) == 16 && (suffix == suffixNone || suffix == suffixUnsigned) {
			suffix |= suffixLong
		}
	case formatBinary:
		if digits != "" && !isBinaryDigit(uint16(digits[0])) {
			if digits[0] == '-' {
				multiplier = -multiplier
			}
			digits = digits[1:]
		}
		v, ok := parseBinary(digits, suffix&suffixUnsigned != 0)
		if !ok {
			return nil, false
		}
		bigValue = v
		if len(digits) == 64 && (suffix == suffixNone || suffix == suffixUnsigned) {
			suffix |= suffixLong
		}
	default:
		v, ok := new(big.Int).SetString(digits, 10)
		if !ok {
			return nil, false
		}
		bigValue = v
	}

	bigValue.Mul(bigValue, big.NewInt(multiplier))
	if suffix != suffixNone {
		return castBig(bigValue, suffix)
	}
	if bigValue.Cmp(bigInt32Min) >= 0 && bigValue.Cmp(bigInt32Max) <= 0 {
		return int32(bigValue.Int64()), true
	}
	if bigValue.Cmp(bigInt64Min) >= 0 && bigValue.Cmp(bigInt64Max) <= 0 {
		return bigValue.Int64(), true
	}
	if format == formatDecimal {
		if new(big.Int).Abs(bigValue).Cmp(bigDecimalMax) <= 0 {
			return Decimal{Unscaled: bigValue, Scale: 0}, true
		}
		f, _ := new(big.Float).SetInt(bigValue).Float64()
		if !math.IsInf(f, 0) {
			return f, true
		}
	}
	return nil, false
}

// parseHexTwosComplement is BigInteger.TryParse with AllowHexSpecifier: the digits are two's complement, so a
// leading digit of 8 or more makes the value negative.
func parseHexTwosComplement(digits string) (*big.Int, bool) {
	if digits == "" {
		return nil, false
	}
	v, ok := new(big.Int).SetString(digits, 16)
	if !ok {
		return nil, false
	}
	if d := digits[0]; (d >= '8' && d <= '9') || (d >= 'a' && d <= 'f') || (d >= 'A' && d <= 'F') {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), uint(4*len(digits))))
	}
	return v, true
}

// parseBinary is Utils.ParseBinary: two's complement only at the sign-bit lengths, and only when the digits fill
// whole bytes, since C# builds the value from big-endian bytes whose partial leading byte has no sign bit.
func parseBinary(digits string, unsigned bool) (*big.Int, bool) {
	if !unsigned {
		if digits == "" {
			// digits[0] throws in C#; the catch makes the literal no number.
			return nil, false
		}
		if digits[0] == '0' {
			unsigned = true
		} else {
			switch n := len(digits); {
			case n == 8, n == 16, n == 32, n == 64, n == 96, n >= 128:
			default:
				unsigned = true
			}
		}
	}
	v := new(big.Int)
	if digits != "" {
		v.SetString(digits, 2)
	}
	if !unsigned && len(digits)%8 == 0 && digits[0] == '1' {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), uint(len(digits))))
	}
	return v, true
}

// decimalPrecision is .NET's DecimalPrecision, and decimalBufferDigits the digits its parser keeps.
const (
	decimalPrecision    = 29
	decimalBufferDigits = 30
)

// parseDecimal is decimal.TryParse with AllowLeadingSign, AllowDecimalPoint and AllowExponent under the
// invariant culture, following .NET's NumberBuffer collection and TryNumberToDecimal rounding.
func parseDecimal(s string) (Decimal, bool) {
	i := 0
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	var digits []byte
	scale := 0
	nonZero := false
	hasNonZeroTail := false
	sawDigit := false
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		sawDigit = true
		if s[i] != '0' || nonZero {
			nonZero = true
			if len(digits) < decimalBufferDigits {
				digits = append(digits, s[i])
			} else if s[i] != '0' {
				hasNonZeroTail = true
			}
			scale++
		}
	}
	if i < len(s) && s[i] == '.' {
		i++
		for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			sawDigit = true
			if s[i] != '0' || nonZero {
				nonZero = true
				if len(digits) < decimalBufferDigits {
					digits = append(digits, s[i])
				} else if s[i] != '0' {
					hasNonZeroTail = true
				}
			} else {
				scale--
			}
		}
	}
	if !sawDigit {
		return Decimal{}, false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		expNeg := false
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			expNeg = s[i] == '-'
			i++
		}
		if i >= len(s) {
			return Decimal{}, false
		}
		exp := 0
		for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			if exp < 100000 {
				exp = exp*10 + int(s[i]-'0')
			}
		}
		if expNeg {
			exp = -exp
		}
		scale += exp
	}
	if i != len(s) {
		return Decimal{}, false
	}
	// Trailing zeros of the digits keep their place for the decimal's scale, as .NET keeps them.
	return numberToDecimal(digits, scale, neg, hasNonZeroTail)
}

// numberToDecimal is TryNumberToDecimal.
func numberToDecimal(digits []byte, e int, neg, hasNonZeroTail bool) (Decimal, bool) {
	p := 0
	at := func(i int) byte {
		if i < len(digits) {
			return digits[i]
		}
		return 0
	}
	c := at(p)
	if c == 0 {
		return Decimal{Unscaled: new(big.Int), Scale: uint8(min(max(-e, 0), 28))}, true
	}
	if e > decimalPrecision {
		return Decimal{}, false
	}
	value := new(big.Int)
	ten := big.NewInt(10)
	limit := new(big.Int).Lsh(big.NewInt(1), 96)
	// Collect digits while the 96-bit mantissa and the 28-digit scale allow, as .NET's two loops do.
	for (e > 0 || (c != 0 && e > -28)) && fitsTimesTenPlus(value, c, limit) {
		value.Mul(value, ten)
		if c != 0 {
			value.Add(value, big.NewInt(int64(c-'0')))
			p++
			c = at(p)
		}
		e--
	}
	if c != 0 && c-'0' >= 5 {
		roundUp := true
		if c == '5' && value.Bit(0) == 0 {
			zeroTail := !hasNonZeroTail
			for q := p + 1; q < len(digits) && zeroTail; q++ {
				zeroTail = digits[q] == '0'
			}
			if zeroTail {
				roundUp = false
			}
		}
		if roundUp {
			value.Add(value, big.NewInt(1))
			if value.Cmp(limit) >= 0 {
				// .NET sets the mantissa to 2^96/10 rounded up and drops one digit of scale.
				value.Add(limit, big.NewInt(9))
				value.Div(value, ten)
				e++
			}
		}
	}
	if e > 0 {
		return Decimal{}, false
	}
	if e <= -decimalPrecision {
		return Decimal{Unscaled: new(big.Int), Scale: decimalPrecision - 1}, true
	}
	if neg {
		value.Neg(value)
	}
	return Decimal{Unscaled: value, Scale: uint8(-e)}, true
}

// fitsTimesTenPlus reports whether value*10 + digit stays below 2^96.
func fitsTimesTenPlus(value *big.Int, c byte, limit *big.Int) bool {
	next := new(big.Int).Mul(value, big.NewInt(10))
	if c != 0 {
		next.Add(next, big.NewInt(int64(c-'0')))
	}
	return next.Cmp(limit) < 0
}

// decimalMultiply is decimal * long: the scale is kept and reduced, rounding half to even, only when the
// mantissa overflows 96 bits; an overflow at scale 0 throws, and the catch makes the literal no number.
func decimalMultiply(d Decimal, m int64) (any, bool) {
	v := new(big.Int).Mul(d.Unscaled, big.NewInt(m))
	scale := d.Scale
	for new(big.Int).Abs(v).Cmp(bigDecimalMax) > 0 {
		if scale == 0 {
			return nil, false
		}
		q, r := new(big.Int).QuoRem(v, big.NewInt(10), new(big.Int))
		r.Abs(r)
		if r.Cmp(big.NewInt(5)) > 0 || (r.Cmp(big.NewInt(5)) == 0 && q.Bit(0) == 1) {
			if v.Sign() < 0 {
				q.Sub(q, big.NewInt(1))
			} else {
				q.Add(q, big.NewInt(1))
			}
		}
		v = q
		scale--
	}
	return Decimal{Unscaled: v, Scale: scale}, true
}
