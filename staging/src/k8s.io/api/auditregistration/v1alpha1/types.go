/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +k8s:openapi-gen=true

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Level defines the amount of information logged during auditing
type Level string

// Valid audit levels
const (
	// LevelNone disables auditing
	LevelNone Level = "None"
	// LevelMetadata provides the basic level of auditing.
	LevelMetadata Level = "Metadata"
	// LevelRequest provides Metadata level of auditing, and additionally
	// logs the request object (does not apply for non-resource requests).
	LevelRequest Level = "Request"
	// LevelRequestResponse provides Request level of auditing, and additionally
	// logs the response object (does not apply for non-resource requests and watches).
	LevelRequestResponse Level = "RequestResponse"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditSink represents a cluster level audit sink
type AuditSink struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// `spec` defines the `auditSink` spec
	Spec AuditSinkSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// AuditSinkSpec holds the spec for the audit sink
type AuditSinkSpec struct {
	// `policy` defines the policy for selecting which events should be sent to the webhook
	// required
	Policy Policy `json:"policy" protobuf:"bytes,1,opt,name=policy"`

	// `webhook` to send events
	// required
	Webhook Webhook `json:"webhook" protobuf:"bytes,2,opt,name=webhook"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditSinkList is a list of AuditSink items.
type AuditSinkList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// `items` is a list of `auditSink` configurations.
	Items []AuditSink `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// Policy defines the configuration of how audit events are logged
type Policy struct {
	// `level` is the base level that all requests are recorded at.
	// available options: None, Metadata, Request, RequestResponse
	// required
	Level Level `json:"level" protobuf:"bytes,1,opt,name=level"`

	// `rules` define how `auditClass` objects should be handled.
	// A request may fall under multiple `auditClass` objects.
	// Rules are evaluated in order (first matching wins).
	// Rules take precedence over the the parent `level`.
	// Unmatched requests use the parent `level` rule.
	// +optional
	Rules []PolicyRule `json:"rules,omitempty" protobuf:"bytes,2,opt,name=rules"`
}

// PolicyRule defines what level is  recorded for an `auditClass`.
type PolicyRule struct {
	// `withAuditClass` is the name of the `auditClass` object. This rule matches requests that are
	// classified with this `auditClass`
	WithAuditClass string `json:"withAuditClass" protobuf:"bytes,1,opt,name=withAuditClass"`

	// `level` that all requests this rule applies to are recorded at.
	// available options: None, Metadata, Request, RequestResponse
	// This will override the parent policy Level.
	// required
	Level Level `json:"level" protobuf:"bytes,2,opt,name=level"`
}

// Webhook holds the configuration of the webhook.
type Webhook struct {
	// `throttle` holds the options for throttling the webhook.
	// +optional
	Throttle *WebhookThrottleConfig `json:"throttle,omitempty" protobuf:"bytes,1,opt,name=throttle"`

	// `clientConfig` holds the connection parameters for the webhook.
	// required
	ClientConfig WebhookClientConfig `json:"clientConfig" protobuf:"bytes,2,opt,name=clientConfig"`
}

// WebhookThrottleConfig holds the configuration for throttling events.
type WebhookThrottleConfig struct {
	// `qps` maximum number of batches per second.
	// default 10 QPS
	// +optional
	QPS *int64 `json:"qps,omitempty" protobuf:"bytes,1,opt,name=qps"`

	// `burst` is the maximum number of events sent at the same moment.
	// default 15 QPS
	// +optional
	Burst *int64 `json:"burst,omitempty" protobuf:"bytes,2,opt,name=burst"`
}

// WebhookClientConfig contains the information to make a connection with the webhook
type WebhookClientConfig struct {
	// `url` gives the location of the webhook, in standard URL form
	// (`scheme://host:port/path`). Exactly one of `url` or `service`
	// must be specified.
	//
	// The `host` should not refer to a service running in the cluster; use
	// the `service` field instead. The host might be resolved via external
	// DNS in some apiservers (e.g., `kube-apiserver` cannot resolve
	// in-cluster DNS as that would be a layering violation). `host` may
	// also be an IP address.
	//
	// Please note that using `localhost` or `127.0.0.1` as a `host` is
	// risky unless you take great care to run this webhook on all hosts
	// which run an apiserver which might need to make calls to this
	// webhook. Such installs are likely to be non-portable, i.e., not easy
	// to turn up in a new cluster.
	//
	// The scheme must be "https"; the URL must begin with "https://".
	//
	// A path is optional, and if present may be any string permissible in
	// a URL. You may use the path to pass an arbitrary string to the
	// webhook, for example, a cluster identifier.
	//
	// Attempting to use a user or basic auth e.g. "user:password@" is not
	// allowed. Fragments ("#...") and query parameters ("?...") are not
	// allowed, either.
	//
	// +optional
	URL *string `json:"url,omitempty" protobuf:"bytes,1,opt,name=url"`

	// `service` is a reference to the service for this webhook. Either
	// `service` or `url` must be specified.
	//
	// If the webhook is running within the cluster, then you should use `service`.
	//
	// +optional
	Service *ServiceReference `json:"service,omitempty" protobuf:"bytes,2,opt,name=service"`

	// `caBundle` is a PEM encoded CA bundle which will be used to validate the webhook's server certificate.
	// If unspecified, system trust roots on the apiserver are used.
	// +optional
	CABundle []byte `json:"caBundle,omitempty" protobuf:"bytes,3,opt,name=caBundle"`
}

// ServiceReference holds a reference to Service.legacy.k8s.io
type ServiceReference struct {
	// `namespace` is the namespace of the service.
	// Required
	Namespace string `json:"namespace" protobuf:"bytes,1,opt,name=namespace"`

	// `name` is the name of the service.
	// Required
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`

	// `path` is an optional URL path which will be sent in any request to
	// this service.
	// +optional
	Path *string `json:"path,omitempty" protobuf:"bytes,3,opt,name=path"`

	// If specified, the port on the service that hosting webhook.
	// Default to 443 for backward compatibility.
	// `port` should be a valid port number (1-65535, inclusive).
	// +optional
	Port *int32 `json:"port,omitempty" protobuf:"varint,4,opt,name=port"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditClass is a set of rules that match a category of requests. Policies 
// can reference and set a level of detail for auditing request categories 
// defined by such AuditClass objects. 
//
// Operations on AuditClass objects should be considered highly privileged, 
// as modifying them affects which requests are audited.
type AuditClass struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// `spec` is the spec for the `auditClass`.
	Spec AuditClassSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditClassList is a list of `auditClass` items.
type AuditClassList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// `items` is a list of `auditClass` objects.
	Items []AuditClass `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// AuditClassSpec is the spec for the `auditClass` object.
type AuditClassSpec struct {
	// 'rules' is the list of rules for matching a category of requests.
	Rules []Rule `json:"rules" protobuf:"bytes,1,rep,name=rules"`
}

// Rule defines a criteria for selecting requests for a class, by matching
// their attributes. On any level in the rule definition selectors that are
// sets are treated as criteria composed with OR, i.e. any successfully 
// matching selector in a set qualifies the whole set as a positive hit. 
// For example, a request for configmap resources is successfully selected
// by a resource selector list that includes configmaps, secrets adn pods 
// altogether. 
type Rule struct {
	// +optional
	BaseSelector `json:",inline"`
	// +optional
	RequestSelector `json:",inline"`
}

// BaseSelector is a criteria for matching requests by common attributes.
type BaseSelector struct {
	// `subjects` is a list of subject names used to match
	// a request subject. Subject types can be `User` and `Group`.
	// +optional
	Subjects []SubjectSelector  `json:"users,omitempty" protobuf:"bytes,1,rep,opt,name=users"`
	// `verbs` is a list of verb names used to match the request verb,
	// e.g. `list`, `create`, `watch` for resource requests or `post`,
	// `put`,`delete` for non-resource requests.
	// An empty list implies any verb.
	// +optional
	Verbs []string `json:"verbs,omitempty" protobuf:"bytes,2,rep,opt,name=verbs"`
}

// SubjectSelector matches requests by a list of subject names for a request 
// subject of a given type (`User` or `Group`).
type SubjectSelector struct {
	// `type` is the type of the subject. Can be `User` or `Group`.
	Type SubjectType `json:"type" protobuf:"bytes,1,name=type"`
	// `names` is the set of subject names sed to match the request subject
	Names []string `json:"names" protobuf:"bytes,2,name=names"`
}
type SubjectType string
const (
	// SubjectTypeUser corresponds to the authenticated 
	// user in a request, e.g. `system:anonymous`.
	SubjectTypeUser = "User"
	// SubjectTypeUserGroup corresponds to the  user group 
	// in a request, e.g. `system:anonymous`.
	SubjectTypeUserGroup  = "Group"
) 

// RequestSelector is a union of different types of request matching criteria lists.
// Only one must be defined at a time, i.e. either `groupResourceSelectors` or 
// `nonResourceSelectors` element is permitted.
// +union
type RequestSelector struct {
	// `groupResourceSelectors` is a list of selectors matching 
	// API group resources.
	// +optional
	GroupResourceSelectors []GroupResourceSelector `json:"groupResourceSelectors,omitempty" protobuf:"bytes,1,rep,opt,name=groupResourceSelectors"`
	// `nonResourceSelectors` is a list of selectors matching 
	// requests that are not related to API resources.
	// +optional
	NonResourceSelectors []NonResourceSelector `json:"nonResourceSelectors,omitempty" protobuf:"bytes,1,rep,opt,name=nonResourceSelectors"`
}

// GroupResourceSelector matches requests for resources in API groups.
type GroupResourceSelector struct {
	// `group` is an API resource group, e.g. `rbac.authorization.k8s.io`.
	// Use "" for the default group.
	// +optional
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	// `resources` is a list of selectors matching various API 
	// resource kinds. If `group` is defined, they must be members
	// of the group.
	// +optional
	Resources []ResourceSelector `json:"resources,omitempty" protobuf:"bytes,2,rep,opt,name=resources"`
	// `scope` indicates the scope of the request - cluster, namespaced or any. 
	// In case of namespaced resource requests, it can specify the namespace too.
	// +optional
	ScopeSelector `json:",inline"`
}

// ResourceSelector matches requests by API resources.
type ResourceSelector struct {
	// `kind` is the resource kind, e.g. "pods"
	Kind string `json:"kind" protobuf:"bytes,1,name=kind"`
	// `subresources` is a list of subresources of this resource `kind`.
	// nil = no subresources.
	// '*' = all subresources & non-subresourced
	// '' = non-subresourced
	// +optional
	Subresources []string `json:"subresources,omitempty" protobuf:"bytes,1,rep,opt,name=subresources"`
	// `objectNames` is a list of names of the objects of this resource kind.
	// +optional
	ObjectNames []string `json:"objectNames,omitempty" protobuf:"bytes,2,rep,opt,name=objectNames"`
}

// ScopeSelector matches requests by API resources scope.
// +union
type ScopeSelector struct {
	// `scope` is scope of a resource. It can be one of `Cluster`, 
	// `Namespaced` or `Any` (default if scope is not defined).
	// +unionDiscriminator
	// +optional 
	Scope ScopeType `json:"scope,omitempty" protobuf:"bytes,1,opt,name=scope"`
	// `namespaces` is a list of namespace selectors
	// +optional
	Namespaces []NamespaceSelector `json:"namespaces,omitempty" protobuf:"bytes,2,rep,opt,name=namespaces"`
}

type ScopeType string

const (
	ScopeTypeAny = "Any" // default
	ScopeTypeCluster = "Cluster"
	ScopeTypeNamespaced = "Namespaced"
)

// NamespaceSelector matches requests by namespace.
type NamespaceSelector struct {
	// `name` is the namespace name
	Name string `json:"name" protobuf:"bytes,1,name=name"`
}

// NonResourceSelector selects requests that do not target API resources.
type NonResourceSelector struct {
	// `urls` is a set of URL paths that should be audited.
	// The `*` is allowed, but only as the full, final step in the path, 
	// and it is delimited by the path separator
	// Examples:
	//  "/metrics" - Log requests for apiserver metrics
	//  "/healthz/*" - Log all health checks
	// +optional
	URLs []string `json:"urls,omitempty" protobuf:"bytes,1,rep,opt,name=urls"`
}