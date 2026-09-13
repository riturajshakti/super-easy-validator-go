package validator

type ErrorCode = string

const (
	CodeRequired        ErrorCode = "REQUIRED"
	CodeDataRequired    ErrorCode = "DATA_REQUIRED"
	CodeDataNotObject   ErrorCode = "DATA_NOT_OBJECT"
	CodeUnexpectedField ErrorCode = "UNEXPECTED_FIELD"

	CodeNotString        ErrorCode = "NOT_STRING"
	CodeNotNumber        ErrorCode = "NOT_NUMBER"
	CodeNotNumericString ErrorCode = "NOT_NUMERIC_STRING"
	CodeNotBoolean       ErrorCode = "NOT_BOOLEAN"
	CodeNotBooleanString ErrorCode = "NOT_BOOLEAN_STRING"
	CodeNotArray         ErrorCode = "NOT_ARRAY"
	CodeNotObject        ErrorCode = "NOT_OBJECT"
	CodeNotBigint        ErrorCode = "NOT_BIGINT"

	CodeNotEmail     ErrorCode = "NOT_EMAIL"
	CodeNotURL       ErrorCode = "NOT_URL"
	CodeNotDomain    ErrorCode = "NOT_DOMAIN"
	CodeNotName      ErrorCode = "NOT_NAME"
	CodeNotFullname  ErrorCode = "NOT_FULLNAME"
	CodeNotUsername  ErrorCode = "NOT_USERNAME"
	CodeNotAlpha     ErrorCode = "NOT_ALPHA"
	CodeNotAlphanum  ErrorCode = "NOT_ALPHANUMERIC"
	CodeNotPhone     ErrorCode = "NOT_PHONE"
	CodeNotPhonecode ErrorCode = "NOT_PHONECODE"
	CodeNotObjectID  ErrorCode = "NOT_OBJECTID"
	CodeNotUUID      ErrorCode = "NOT_UUID"
	CodeNotDate      ErrorCode = "NOT_DATE"
	CodeNotDateonly  ErrorCode = "NOT_DATEONLY"
	CodeNotTime      ErrorCode = "NOT_TIME"
	CodeNotIP        ErrorCode = "NOT_IP"

	CodeNotLowercase ErrorCode = "NOT_LOWERCASE"
	CodeNotUppercase ErrorCode = "NOT_UPPERCASE"

	CodeNotInteger  ErrorCode = "NOT_INTEGER"
	CodeNotPositive ErrorCode = "NOT_POSITIVE"
	CodeNotNegative ErrorCode = "NOT_NEGATIVE"
	CodeNotNatural  ErrorCode = "NOT_NATURAL"
	CodeNotWhole    ErrorCode = "NOT_WHOLE"

	CodeLengthMismatch ErrorCode = "LENGTH_MISMATCH"
	CodeDigitsMismatch ErrorCode = "DIGITS_MISMATCH"
	CodeTooShort       ErrorCode = "TOO_SHORT"
	CodeTooLong        ErrorCode = "TOO_LONG"
	CodeTooSmall       ErrorCode = "TOO_SMALL"
	CodeTooLarge       ErrorCode = "TOO_LARGE"
	CodeDateTooEarly   ErrorCode = "DATE_TOO_EARLY"
	CodeDateTooLate    ErrorCode = "DATE_TOO_LATE"
	CodeNotEqual       ErrorCode = "NOT_EQUAL"

	CodeDecimalSizeMismatch ErrorCode = "DECIMAL_SIZE_MISMATCH"
	CodeDecimalTooFew       ErrorCode = "DECIMAL_TOO_FEW"
	CodeDecimalTooMany      ErrorCode = "DECIMAL_TOO_MANY"
	CodeNotANumber          ErrorCode = "NOT_A_NUMBER"

	CodeEnumMismatch  ErrorCode = "ENUM_MISMATCH"
	CodeRegexMismatch ErrorCode = "REGEX_MISMATCH"

	CodeAtleastNotMet   ErrorCode = "ATLEAST_NOT_MET"
	CodeAtmostExceeded  ErrorCode = "ATMOST_EXCEEDED"
	CodeNoBranchMatched ErrorCode = "NO_BRANCH_MATCHED"
	CodeCustomRuleFail  ErrorCode = "CUSTOM_RULE_FAILED"
	CodeNoCaseMatched   ErrorCode = "NO_CASE_MATCHED"

	CodeInvalidRule   ErrorCode = "INVALID_RULE"
	CodeInternalError ErrorCode = "INTERNAL_ERROR"
)
