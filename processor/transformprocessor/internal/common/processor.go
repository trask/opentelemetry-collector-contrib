// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

var _ baseContext = &resourceStatements{}

type resourceStatements struct {
	ottl.StatementSequence[ottlresource.TransformContext]
	expr.BoolExpr[ottlresource.TransformContext]
}

func (r resourceStatements) Context() ContextID {
	return Resource
}

func (r resourceStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for _, rspans := range td.ResourceSpans().All() {
		tCtx := ottlresource.NewTransformContext(rspans.Resource(), rspans)
		condition, err := r.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := r.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r resourceStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for _, rmetrics := range md.ResourceMetrics().All() {
		tCtx := ottlresource.NewTransformContext(rmetrics.Resource(), rmetrics)
		condition, err := r.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := r.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r resourceStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for _, rlogs := range ld.ResourceLogs().All() {
		tCtx := ottlresource.NewTransformContext(rlogs.Resource(), rlogs)
		condition, err := r.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := r.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r resourceStatements) ConsumeProfiles(ctx context.Context, ld pprofile.Profiles) error {
	for _, rprofiles := range ld.ResourceProfiles().All() {
		tCtx := ottlresource.NewTransformContext(rprofiles.Resource(), rprofiles)
		condition, err := r.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := r.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var _ baseContext = &scopeStatements{}

type scopeStatements struct {
	ottl.StatementSequence[ottlscope.TransformContext]
	expr.BoolExpr[ottlscope.TransformContext]
}

func (s scopeStatements) Context() ContextID {
	return Scope
}

func (s scopeStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for _, rspans := range td.ResourceSpans().All() {
		for _, sspans := range rspans.ScopeSpans().All() {
			tCtx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource(), sspans)
			condition, err := s.Eval(ctx, tCtx)
			if err != nil {
				return err
			}
			if condition {
				err := s.Execute(ctx, tCtx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s scopeStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for _, rmetrics := range md.ResourceMetrics().All() {
		for _, smetrics := range rmetrics.ScopeMetrics().All() {
			tCtx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource(), smetrics)
			condition, err := s.Eval(ctx, tCtx)
			if err != nil {
				return err
			}
			if condition {
				err := s.Execute(ctx, tCtx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s scopeStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for _, rlogs := range ld.ResourceLogs().All() {
		for _, slogs := range rlogs.ScopeLogs().All() {
			tCtx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource(), slogs)
			condition, err := s.Eval(ctx, tCtx)
			if err != nil {
				return err
			}
			if condition {
				err := s.Execute(ctx, tCtx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s scopeStatements) ConsumeProfiles(ctx context.Context, ld pprofile.Profiles) error {
	for _, rprofiles := range ld.ResourceProfiles().All() {
		for _, sprofiles := range rprofiles.ScopeProfiles().All() {
			tCtx := ottlscope.NewTransformContext(sprofiles.Scope(), rprofiles.Resource(), sprofiles)
			condition, err := s.Eval(ctx, tCtx)
			if err != nil {
				return err
			}
			if condition {
				err := s.Execute(ctx, tCtx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type baseContext interface {
	TracesConsumer
	MetricsConsumer
	LogsConsumer
	ProfilesConsumer
}

func withCommonContextParsers[R any]() ottl.ParserCollectionOption[R] {
	return func(pc *ottl.ParserCollection[R]) error {
		rp, err := ottlresource.NewParser(ResourceFunctions(), pc.Settings, ottlresource.EnablePathContextNames())
		if err != nil {
			return err
		}
		sp, err := ottlscope.NewParser(ScopeFunctions(), pc.Settings, ottlscope.EnablePathContextNames())
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(ottlresource.ContextName, &rp, ottl.WithStatementConverter[ottlresource.TransformContext, R](parseResourceContextStatements))(pc)
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(ottlscope.ContextName, &sp, ottl.WithStatementConverter[ottlscope.TransformContext, R](parseScopeContextStatements))(pc)
		if err != nil {
			return err
		}

		return nil
	}
}

func parseResourceContextStatements[R any](
	pc *ottl.ParserCollection[R],
	statements ottl.StatementsGetter,
	parsedStatements []*ottl.Statement[ottlresource.TransformContext],
) (R, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return *new(R), err
	}
	errorMode := pc.ErrorMode
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[ottlresource.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlresource.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForResourceWithOptions, contextStatements.Conditions, errorMode, pc.Settings, filterottl.StandardResourceFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return *new(R), errGlobalBoolExpr
	}
	rStatements := ottlresource.NewStatementSequence(parsedStatements, pc.Settings, ottlresource.WithStatementSequenceErrorMode(errorMode))
	result := (baseContext)(resourceStatements{rStatements, globalExpr})
	return result.(R), nil
}

func parseScopeContextStatements[R any](
	pc *ottl.ParserCollection[R],
	statements ottl.StatementsGetter,
	parsedStatements []*ottl.Statement[ottlscope.TransformContext],
) (R, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return *new(R), err
	}
	errorMode := pc.ErrorMode
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[ottlscope.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlscope.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForScopeWithOptions, contextStatements.Conditions, errorMode, pc.Settings, filterottl.StandardScopeFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return *new(R), errGlobalBoolExpr
	}
	sStatements := ottlscope.NewStatementSequence(parsedStatements, pc.Settings, ottlscope.WithStatementSequenceErrorMode(errorMode))
	result := (baseContext)(scopeStatements{sStatements, globalExpr})
	return result.(R), nil
}

func parseGlobalExpr[K any, O any](
	boolExprFunc func([]string, map[string]ottl.Factory[K], ottl.ErrorMode, component.TelemetrySettings, []O) (*ottl.ConditionSequence[K], error),
	conditions []string,
	errorMode ottl.ErrorMode,
	settings component.TelemetrySettings,
	standardFuncs map[string]ottl.Factory[K],
	parserOptions []O,
) (expr.BoolExpr[K], error) {
	if len(conditions) > 0 {
		return boolExprFunc(conditions, standardFuncs, errorMode, settings, parserOptions)
	}
	// By default, set the global expression to always true unless conditions are specified.
	return expr.AlwaysTrue[K](), nil
}
