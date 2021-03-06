/*
Package oplog implements standard (but extensible) logging for operations (events with status, start/end times).
Useful for operational queries and dashboarding with CloudWatch Insights/Metrics. Using standard attributes
describing operations and their status allows easy creation of Cloudwatch alarms for discrete system operations.
The 3 level (namespace, component, operation) hierarchy enables grouping when graphing/querying. For
example, if the hierarchy has top level namespace of "logprocessor" then you can see all errors
where namespace="logprocessor" in single graph/query. Similarly you can compute latency and other
performance related metrics in aggregate over different _standard_ dimensions.

Example usage:

  manager := oplog.NewManager("panther", "logprocessor")
  // record every S3 object read
  operation := manager.Start("readlogfile")
  defer func() {
		operation.Stop()
        operation.Log(err,
           zap.String("bucket", bucket),
           zap.String("object", object))
  }()
  ... code to read log from S3

*/
package oplog

/**
 * Copyright 2020 Panther Labs Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	"time"

	"go.uber.org/zap"
)

/* TODO: Consider emitting CW embedded metric format also:
https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Specification.html
*/

const (
	Success = "success"
	Failure = "failure"
)

type Manager struct {
	Namespace string
	Component string
}

func NewManager(namespace, component string) *Manager {
	return &Manager{
		Namespace: namespace,
		Component: component,
	}
}

type Operation struct {
	Manager    *Manager
	Name       string
	Dimensions []zap.Field
	StartTime  time.Time
	EndTime    time.Time
}

func (m *Manager) Start(operation string, dimensions ...zap.Field) *Operation {
	return &Operation{
		Manager:    m,
		Name:       operation,
		Dimensions: dimensions,
		StartTime:  time.Now().UTC(),
	}
}

func (o *Operation) Stop() {
	o.EndTime = time.Now().UTC()
}

func (o *Operation) zapMsg() string {
	return o.Manager.Namespace + ":" + o.Manager.Component + ":" + o.Name
}

func (o *Operation) fields(status string) []zap.Field {
	return append(o.standardFields(status), o.Dimensions...)
}

func (o *Operation) standardFields(status string) (fields []zap.Field) {
	var dur time.Duration
	if o.EndTime.IsZero() { // operation is still going
		dur = time.Since(o.StartTime)
	} else {
		dur = o.EndTime.Sub(o.StartTime)
	}
	fields = []zap.Field{
		zap.String("namespace", o.Manager.Namespace),
		zap.String("component", o.Manager.Component),
		zap.String("operation", o.Name),
		zap.String("status", status),
		zap.Time("startOp", o.StartTime),
		zap.Duration("opTime", dur),
	}
	if !o.EndTime.IsZero() {
		fields = append(fields, zap.Time("endOp", o.EndTime))
	}
	return
}

// wrapper handling err
func (o *Operation) Log(err error, fields ...zap.Field) {
	if err != nil {
		o.LogError(err, fields...)
	} else {
		o.LogSuccess(fields...)
	}
}

func (o *Operation) LogSuccess(fields ...zap.Field) {
	zap.L().Info(o.zapMsg(), append(fields, o.fields(Success)...)...)
}

// implies status=Fail
func (o *Operation) LogWarn(err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	zap.L().Warn(o.zapMsg(), append(fields, o.fields(Failure)...)...)
}

// implies status=Fail
func (o *Operation) LogError(err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	zap.L().Error(o.zapMsg(), append(fields, o.fields(Failure)...)...)
}
