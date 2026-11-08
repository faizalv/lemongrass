// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// Pipelines, pipeline chains, redirections and commands with their arguments: the Pipelines region of Parser.cs.
// Where C# restores tokenizer state in a finally block, the restore here is a plain statement after the call: a
// parse error aborts the whole parse, so nothing reads the state after one.

// verbatimArgument is VERBATIM_ARGUMENT, the --% that passes the rest of the line to a native command as written.
const verbatimArgument = "--%"

// commandArgumentContext is CommandArgumentContext.
type commandArgumentContext int

const (
	argCommandName                        commandArgumentContext = 0x01
	argCommandNameAfterInvocationOperator commandArgumentContext = 0x02 | argCommandName
	argFileName                           commandArgumentContext = 0x04
	argCommandArgument                    commandArgumentContext = 0x08
	argSwitchCondition                    commandArgumentContext = 0x10
)

// pipelineChainRule is PipelineChainRule: an assignment, or pipelines joined by && and ||, with a trailing & for a
// background job.
func (p *parser) pipelineChainRule() PipelineBaseAst {
	p.enter()
	defer p.leave()

	var assignToken *Token
	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeExpression)
	expr := p.expressionRule(false)
	if expr != nil {
		t := p.peekToken()
		if t.Kind.HasTrait(FlagAssignmentOperator) {
			p.skipToken()
			assignToken = t
		}
	}
	p.setTokenizerMode(oldMode)

	if expr != nil && assignToken != nil {
		p.skipNewlines()
		statement := p.statementRule()
		if statement == nil {
			p.reportIncompleteInput(after(assignToken.Extent), "ExpectedValueExpression", assignToken.Kind.Text())
		}
		return newAssignmentStatementAst(extentOf(expr.Extent(), statement.Extent()), expr, assignToken.Kind, statement, assignToken.Extent)
	}

	startExpression := expr
	var currentPipelineChain ChainableAst
	var currentChainOperatorToken *Token
	background := false
	for {
		var nextPipeline *PipelineAst
		var firstPipelineToken *Token
		if startExpression != nil {
			nextPipeline, _ = p.pipelineRule(startExpression, false).(*PipelineAst)
			startExpression = nil
		} else {
			firstPipelineToken = p.peekToken()
			nextPipeline, _ = p.pipelineRule(nil, false).(*PipelineAst)
		}

		if nextPipeline == nil {
			if currentChainOperatorToken == nil {
				return nil
			}
			switch firstPipelineToken.Kind {
			case EndOfInput:
				p.reportIncompleteInput(after(currentChainOperatorToken.Extent), "EmptyPipelineChainElement", currentChainOperatorToken.Text)
			case Dot, Ampersand:
				// C# returns an error statement without reporting here; commandRule has already reported
				// MissingExpression for a lone & or ., so this is never reached.
				p.reportError(extentOf(currentPipelineChain.Extent(), currentChainOperatorToken.Extent), ErrUnreachable)
			default:
				p.reportError(extentOf(currentChainOperatorToken.Extent, firstPipelineToken.Extent), "EmptyPipelineChainElement", currentChainOperatorToken.Text)
			}
		}

		nextToken := p.peekToken()
		chained := false
		switch nextToken.Kind {
		case AndAnd, OrOr:
			p.skipToken()
			p.skipNewlines()
			chained = true
		case Ampersand:
			p.skipToken()
			if following := p.peekToken(); following.Kind == AndAnd || following.Kind == OrOr {
				p.skipToken()
				p.reportError(following.Extent, "BackgroundOperatorInPipelineChain")
			}
			background = true
		}

		if !chained {
			if currentPipelineChain == nil {
				if background {
					nextPipeline.Background = true
				}
				return nextPipeline
			}
			return newPipelineChainAst(extentOf(currentPipelineChain.Extent(), nextPipeline.Extent()), currentPipelineChain, nextPipeline,
				currentChainOperatorToken.Kind, background)
		}

		if currentPipelineChain == nil {
			currentPipelineChain = nextPipeline
		} else {
			currentPipelineChain = newPipelineChainAst(extentOf(currentPipelineChain.Extent(), nextPipeline.Extent()), currentPipelineChain,
				nextPipeline, currentChainOperatorToken.Kind, false)
		}
		currentChainOperatorToken = nextToken

		if p.peekToken().Kind == EndOfInput {
			p.reportIncompleteInput(after(nextToken.Extent), "EmptyPipelineChainElement")
		}
	}
}

// pipelineRule is PipelineRule: commands and expressions joined by |. It returns nil, never a typed nil, when no
// pipeline starts here.
func (p *parser) pipelineRule(startExpression ExpressionAst, allowBackground bool) PipelineBaseAst {
	p.enter()
	defer p.leave()

	var elements []CommandBaseAst
	var nextToken *Token
	background := false
	expr := startExpression
	for scanning := true; scanning; {
		var command CommandBaseAst

		if expr == nil {
			oldMode := p.tok.Mode
			p.setTokenizerMode(ModeExpression)
			expr = p.expressionRule(false)
			p.setTokenizerMode(oldMode)
		}

		if expr != nil {
			if len(elements) > 0 {
				p.reportError(expr.Extent(), "ExpressionsMustBeFirstInPipeline")
			}
			var redirections *[maxRedirections]RedirectionAst
			var lastRedirection RedirectionAst
			for t := p.peekToken(); t.isRedirection(); t = p.peekToken() {
				p.skipToken()
				if redirections == nil {
					redirections = new([maxRedirections]RedirectionAst)
				}
				var unused Extent
				lastRedirection = p.redirectionRule(t, redirections, &unused)
			}
			exprExtent := expr.Extent()
			if lastRedirection != nil {
				exprExtent = extentOf(expr.Extent(), lastRedirection.Extent())
			}
			command = newCommandExpressionAst(exprExtent, expr, collectRedirections(redirections))
		} else if c, ok := p.commandRule(false).(*CommandAst); ok {
			command = c
		}

		if command != nil {
			elements = append(elements, command)
		} else if len(elements) > 0 || p.peekToken().Kind == Pipe {
			// Peek only where C# does: a peek can scan a token whose own error would be reported first.
			if nextToken != nil {
				p.reportIncompleteInput(after(nextToken.Extent), "EmptyPipeElement")
			}
			p.reportIncompleteInput(p.peekToken().Extent, "EmptyPipeElement")
		}

		expr = nil
		nextToken = p.peekToken()

		// A newline followed by | on the next line continues the pipeline.
		if nextToken.Kind == NewLine && p.tok.isPipeContinuation(nextToken.Extent) {
			p.skipNewlines()
			nextToken = p.peekToken()
		}

		switch nextToken.Kind {
		case Semi, NewLine, RParen, RCurly, EndOfInput, AndAnd, OrOr:
			scanning = false
		case Ampersand:
			scanning = false
			if allowBackground {
				p.skipToken()
				background = true
			}
		case Pipe:
			p.skipToken()
			p.skipNewlines()
			if p.peekToken().Kind == EndOfInput {
				p.reportIncompleteInput(after(nextToken.Extent), "EmptyPipeElement")
			}
		default:
			p.reportError(nextToken.Extent, "UnexpectedToken", nextToken.Text)
		}
	}

	if len(elements) == 0 {
		return nil
	}
	return newPipelineAst(extentOf(elements[0].Extent(), elements[len(elements)-1].Extent()), elements, background)
}

// maxRedirections is CommandBaseAst.MaxRedirections, one slot per RedirectionStream.
const maxRedirections = int(StreamInformation) + 1

// collectRedirections is redirections?.Where(r => r != null): the redirections in stream order, not source order.
func collectRedirections(redirections *[maxRedirections]RedirectionAst) []RedirectionAst {
	if redirections == nil {
		return nil
	}
	var out []RedirectionAst
	for _, r := range redirections {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}

// redirectionStreamNames are the ParserStrings resources RedirectionRule names a stream by, indexed by stream.
var redirectionStreamNames = [maxRedirections]string{
	"AllStream", "OutputStream", "ErrorStream", "WarningStream", "VerboseStream", "DebugStream", "InformationStream",
}

// redirectionRule is RedirectionRule; redirections holds the redirections seen so far, indexed by stream, and
// extent receives the redirection's extent.
func (p *parser) redirectionRule(redirectionToken *Token, redirections *[maxRedirections]RedirectionAst, extent *Extent) RedirectionAst {
	var result RedirectionAst
	if redirectionToken.Class != ClassMergingRedirectionToken {
		filename := p.getSingleCommandArgument(argFileName)
		if filename == nil {
			p.reportError(after(redirectionToken.Extent), "MissingFileSpecification")
		}
		if redirectionToken.Class == ClassInputRedirectionToken {
			p.reportError(redirectionToken.Extent, "RedirectionNotSupported", redirectionToken.Text)
		}
		result = newFileRedirectionAst(extentOf(redirectionToken.Extent, filename.Extent()), redirectionToken.FromStream, filename,
			redirectionToken.Append)
	} else {
		fromStream, toStream := redirectionToken.FromStream, redirectionToken.ToStream
		if toStream != StreamOutput || fromStream == toStream {
			p.reportError(redirectionToken.Extent, "RedirectionNotSupported", redirectionToken.Text)
		}
		result = newMergingRedirectionAst(redirectionToken.Extent, fromStream, toStream)
	}

	from := result.GetFromStream()
	if redirections[from] != nil {
		p.reportError(result.Extent(), "StreamAlreadyRedirected", parserStrings[redirectionStreamNames[from]])
	}
	redirections[from] = result
	*extent = result.Extent()
	return result
}

// getSingleCommandArgument is GetSingleCommandArgument: one argument read in command mode.
func (p *parser) getSingleCommandArgument(context commandArgumentContext) ExpressionAst {
	if k := p.peekToken().Kind; k == Comma || k == EndOfInput {
		return nil
	}
	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeCommand)
	arg := p.getCommandArgument(context, p.nextToken())
	p.setTokenizerMode(oldMode)
	return arg
}

// getCommandArgument is GetCommandArgument: a command name or argument starting at t. As an argument, values
// joined by commas form an array literal. It returns nil when t starts no argument, leaving t pushed back.
func (p *parser) getCommandArgument(context commandArgumentContext, t *Token) ExpressionAst {
	var exprAst ExpressionAst
	var commandArgs []ExpressionAst
	var commaToken *Token
	foundVerbatimArgument := false

	for {
		switch t.Kind {
		case Pipe, RCurly, RParen, EndOfInput, NewLine, Semi, Redirection, RedirectInStd, AndAnd, OrOr, Ampersand, MinusMinus, Comma:
			p.ungetToken(t)
			if commaToken == nil {
				return nil
			}
			p.reportIncompleteInput(after(commaToken.Extent), "MissingExpression", ",")

		case SplattedVariable, Variable, Number, HereStringExpandable, StringExpandable, HereStringLiteral, StringLiteral,
			LParen, DollarParen, AtParen, AtCurly, LCurly:
			p.ungetToken(t)
			exprAst = p.primaryExpressionRule(true)

		case Generic:
			if context&argCommandName != 0 {
				t.Flags |= FlagCommandName
			}
			// A bare word holding a variable expands, except as a plain command name; after & or . it expands too.
			if t.isStringExpandable() && context != argCommandName {
				exprAst = newExpandableStringExpressionAst(t, t.StringValue, t.FormatString, p.parseNestedExpressions(t))
			} else {
				exprAst = newStringConstantExpressionAst(t.Extent, t.StringValue, BareWord)
				if t.StringValue == verbatimArgument {
					foundVerbatimArgument = true
				}
			}

		default:
			// A keyword or operator token read as a command word is a bare word.
			exprAst = newStringConstantExpressionAst(t.Extent, t.Text, BareWord)
			t.Flags &^= FlagKeyword
			switch context {
			case argCommandName, argCommandNameAfterInvocationOperator:
				t.Flags |= FlagCommandName
			case argFileName, argCommandArgument, argSwitchCondition:
				t.setIsCommandArgument()
			}
		}

		if context != argCommandArgument || foundVerbatimArgument {
			break
		}
		t = p.peekToken()
		if t.Kind != Comma {
			break
		}
		commaToken = t
		commandArgs = append(commandArgs, exprAst)
		p.skipToken()
		p.skipNewlines()
		t = p.nextToken()
	}

	if commandArgs != nil {
		commandArgs = append(commandArgs, exprAst)
		return newArrayLiteralAst(extentOf(commandArgs[0].Extent(), commandArgs[len(commandArgs)-1].Extent()), commandArgs)
	}
	return exprAst
}

// commandRule is CommandRule: a command name, optionally after & or ., with its parameters, arguments and
// redirections. It returns a *CommandAst, or nil, never a typed nil, when no command starts here.
func (p *parser) commandRule(forDynamicKeyword bool) Ast {
	p.enter()
	defer p.leave()

	sawDashDash := false
	foundVerbatimArgument := false
	var redirections *[maxRedirections]RedirectionAst
	var elements []CommandElementAst

	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeCommand)

	t := p.nextToken()
	firstToken := t
	endExtent := t.Extent
	dotSource, ampersand := false, false

	var context commandArgumentContext
	switch t.Kind {
	case Dot:
		dotSource = true
		t = p.nextToken()
		context = argCommandNameAfterInvocationOperator
	case Ampersand:
		ampersand = true
		t = p.nextToken()
		context = argCommandNameAfterInvocationOperator
	default:
		context = argCommandName
	}

	for scanning := true; scanning; {
		switch t.Kind {
		case Pipe, RCurly, RParen, EndOfInput, NewLine, Semi, AndAnd, OrOr, Ampersand:
			p.ungetToken(t)
			scanning = false
			continue

		case MinusMinus:
			endExtent = t.Extent
			// The first -- ends parameter parsing and is kept as a parameter named "-"; any later one is an argument.
			if sawDashDash {
				elements = append(elements, newStringConstantExpressionAst(t.Extent, t.Text, BareWord))
			} else {
				elements = append(elements, newCommandParameterAst(t.Extent, "-", nil, t.Extent))
			}
			sawDashDash = true

		case Comma:
			p.reportError(t.Extent, "MissingArgument")

		case Parameter:
			if context&argCommandName != 0 || sawDashDash {
				endExtent = t.Extent
				t.Flags |= FlagCommandName
				elements = append(elements, newStringConstantExpressionAst(t.Extent, t.Text, BareWord))
				break
			}
			var parameterArgs ExpressionAst
			extent := t.Extent
			if t.UsedColon && p.peekToken().Kind != Comma {
				parameterArgs = p.getCommandArgument(argCommandArgument, p.nextToken())
				if parameterArgs == nil {
					p.reportError(after(extent), "ParameterRequiresArgument", t.Text)
				}
				extent = extentOf(t.Extent, parameterArgs.Extent())
			}
			endExtent = extent
			elements = append(elements, newCommandParameterAst(extent, t.ParameterName, parameterArgs, t.Extent))

		case Redirection, RedirectInStd:
			if context&argCommandName == 0 {
				if redirections == nil {
					redirections = new([maxRedirections]RedirectionAst)
				}
				p.redirectionRule(t, redirections, &endExtent)
			} else {
				endExtent = t.Extent
				elements = append(elements, newStringConstantExpressionAst(t.Extent, t.Text, BareWord))
			}

		default:
			if t.Kind == InlineScript && context == argCommandName {
				scanning = p.inlineScriptRule(t, &elements)
				endExtent = elements[len(elements)-1].Extent()
				if !scanning {
					continue
				}
				break
			}
			ast := p.getCommandArgument(context, t)
			// --% as a string token, quoted or not, makes the rest of the line one verbatim argument.
			if t.isStringToken() && t.StringValue == verbatimArgument {
				elements = append(elements, ast)
				endExtent = ast.Extent()
				if verbatimToken := p.getVerbatimCommandArgumentToken(); verbatimToken != nil {
					foundVerbatimArgument = true
					scanning = false
					ast = newStringConstantExpressionAst(verbatimToken.Extent, verbatimToken.StringValue, BareWord)
					elements = append(elements, ast)
					endExtent = ast.Extent()
				}
				break
			}
			endExtent = ast.Extent()
			elements = append(elements, ast)
		}

		if !foundVerbatimArgument {
			context = argCommandArgument
			t = p.nextToken()
		}
	}
	p.setTokenizerMode(oldMode)

	if len(elements) == 0 {
		if dotSource || ampersand {
			p.reportError(firstToken.Extent, "MissingExpression", firstToken.Text)
		}
		return nil
	}
	if forDynamicKeyword {
		// DynamicKeywordStatementRule is not ported (contract decision 5), so nothing asks for this.
		p.reportError(firstToken.Extent, ErrUnreachable)
	}

	invocationOperator := Unknown
	if dotSource || ampersand {
		invocationOperator = firstToken.Kind
	}
	return newCommandAst(extentOf(firstToken.Extent, endExtent), elements, invocationOperator, collectRedirections(redirections))
}
