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

package dynamic

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	auditregv1alpha1 "k8s.io/api/auditregistration/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/audit"
	webhook "k8s.io/apiserver/pkg/util/webhook"
	informers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	core "k8s.io/client-go/testing"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
	testPolicy         = auditregv1alpha1.Policy{
		Level: auditregv1alpha1.LevelMetadata,
		Stages: []auditregv1alpha1.Stage{
			auditregv1alpha1.StageResponseStarted,
		},
	}
)

// testReconciler is a helper struct for test context aware
// reconcilliation functions (`testReconcileF`) that can be
// injected ot controller as `reconciler`
type testReconciler struct {
	t                *testing.T
	controller       *defaultAuditSinkControl
	callStackChannel chan string
	testReconcileF   func(tester *testReconciler, key uidKey) error
}

// reconcile delegates to the supplied `testReconcileF` suppliing the text context
func (r *testReconciler) reconcile(key uidKey) error {
	return r.testReconcileF(r, key)
}

// testDelegateManager is a helper struct defining a call channel that will
// be signaled upon a delegate manager method invocation. Useful for testing
// call expectations and recieving immediate feedback upon operations (stack)
// completed
type testDelegateManager struct {
	original   delegateManager
	callbackCh chan string
}

// an implementaiton of `registerAndRunNewDelegate` that delegates to the original
// method and then signals its completetion to `callbackCh`
func (s *testDelegateManager) registerAndRunNewDelegate(sink *auditregv1alpha1.AuditSink) error {
	err := s.original.registerAndRunNewDelegate(sink)
	s.callbackCh <- "registerAndRunNewDelegate"
	return err
}

// an implementaiton of `unregisterAndStopDelegate` that delegates to the original
// method and then signals its completetion to `callbackCh`
func (s *testDelegateManager) unregisterAndStopDelegate(uid types.UID) {
	s.original.unregisterAndStopDelegate(uid)
	s.callbackCh <- "unregisterAndStopDelegate"
}

var errWaitCondition error

// blocks waiting on multiple signals on channel in order, until all expected signals are received in order,
// the channel is closed, or a timeout
func waitForCallStack(stack []string, callStackCh chan string, timeout time.Duration) error {
	callstack := []string{}
	for {
		select {
		case <-time.After(timeout):
			return wait.ErrWaitTimeout
		case sig, open := <-callStackCh:
			if !open {
				return fmt.Errorf("trying to send to a closed channel")
			}
			if sig != "" {
				callstack = append(callstack, sig)
				if reflect.DeepEqual(stack, callstack) {
					return nil
				}
			}
		}
	}
	return errWaitCondition
}

// defaultTestConfig returns a Config object suitable for testing along with its
// associated stopChan
func defaultTestConfig() *Config {
	authWrapper := webhook.AuthenticationInfoResolverWrapper(
		func(a webhook.AuthenticationInfoResolver) webhook.AuthenticationInfoResolver { return a },
	)
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, noResyncPeriodFunc())

	eventSink := &v1core.EventSinkImpl{Interface: client.CoreV1().Events("")}
	batchConfig := NewDefaultWebhookBatchConfig()
	batchConfig.MaxBatchSize = 1
	return &Config{
		AuditSinkInformer:  informerFactory.Auditregistration().V1alpha1().AuditSinks(),
		AuditClassInformer: informerFactory.Auditregistration().V1alpha1().AuditClasses(),
		EventConfig:        EventConfig{Sink: eventSink},
		BufferedConfig:     batchConfig,
		WebhookConfig: WebhookConfig{
			AuthInfoResolverWrapper: authWrapper,
			ServiceResolver:         webhook.NewDefaultServiceResolver(),
		},
		WorkersCount: 2,
	}
}

// newAuditSink is a factory function for AuditSink resources
func newAuditSink(name string, webhookurl *string, policy auditregv1alpha1.Policy) *auditregv1alpha1.AuditSink {
	sink := &auditregv1alpha1.AuditSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  uuid.NewUUID(),
		},
		Spec: auditregv1alpha1.AuditSinkSpec{
			Policy: policy,
			Webhook: auditregv1alpha1.Webhook{
				ClientConfig: auditregv1alpha1.WebhookClientConfig{
					URL: webhookurl,
				},
			},
		},
	}
	return sink
}

// newAuditClass is a factory function for AuditClasses
func newAuditClass(name string, selectors []auditregv1alpha1.RequestSelector) *auditregv1alpha1.AuditClass {
	return &auditregv1alpha1.AuditClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  uuid.NewUUID(),
		},
		Spec: auditregv1alpha1.AuditClassSpec{
			RequestSelectors: selectors,
		},
	}
}

// bindSinkToClass adds a reference to a class in a sink or updates it if it already exists
func bindSinkToClass(sink *auditregv1alpha1.AuditSink, className string, level auditregv1alpha1.Level, stages ...auditregv1alpha1.Stage) {
	if level == "" {
		level = auditregv1alpha1.LevelMetadata
	}
	stagesArr := append([]auditregv1alpha1.Stage{}, stages...)
	var existingBindingRule auditregv1alpha1.PolicyRule
	for _, rule := range sink.Spec.Policy.Rules {
		if rule.WithAuditClass == className {
			existingBindingRule = rule
			break
		}
	}
	// create dynamically an empty copy of the struct object to compare against. Consider changing this with static zero struct
	var empty auditregv1alpha1.PolicyRule //reflect.New(reflect.TypeOf(existingBindingRule)).Elem().Interface()
	if !reflect.DeepEqual(existingBindingRule, empty) {
		existingBindingRule.Level = level
		existingBindingRule.Stages = stagesArr
	} else {
		sink.Spec.Policy.Rules = append(sink.Spec.Policy.Rules, auditregv1alpha1.PolicyRule{
			Level:          level,
			Stages:         stagesArr,
			WithAuditClass: className,
		})
	}
}

// newTestInformerFactory creates a shared informer factory and initializes it with `preloadedAuditObjects` if any (AuditClass/AuditSink).
// A fake watcher is created for each `watchedresource` argument ('auditsinks) and added to the fake clientset used by the factory.
// The function returns a fake watchers that can be used to simulate resource change events sent from the api server and the clientset used to
// initialize the factory.
func newTestInformerFactory(preloadedAuditObjects []runtime.Object, watchedResources ...string) (informers.SharedInformerFactory, map[string]*watch.FakeWatcher, clientset.Interface) {
	clientset := fake.NewSimpleClientset(preloadedAuditObjects...)
	watchers := make(map[string]*watch.FakeWatcher)
	for _, resource := range watchedResources {
		watchers[resource] = watch.NewFake()
		clientset.PrependWatchReactor(resource, core.DefaultWatchReactor(watchers[resource], nil))
	}
	informerFactory := informers.NewSharedInformerFactory(clientset, noResyncPeriodFunc())
	for _, obj := range preloadedAuditObjects {
		switch obj.(type) {
		case *auditregv1alpha1.AuditSink:
			informerFactory.Auditregistration().V1alpha1().AuditSinks().Informer().GetIndexer().Add(obj)
		case *auditregv1alpha1.AuditClass:
			informerFactory.Auditregistration().V1alpha1().AuditClasses().Informer().GetIndexer().Add(obj)
		default:
			panic("unknown type")
		}
	}
	return informerFactory, watchers, clientset
}

// configForInformerFactory creates a test configuration for dynamic backends with the supplied informerFactory
// The configuration reduces buffering settings for immediate processing.
func configForInformerFactory(informerFactory informers.SharedInformerFactory, client clientset.Interface) *Config {
	authWrapper := webhook.AuthenticationInfoResolverWrapper(
		func(a webhook.AuthenticationInfoResolver) webhook.AuthenticationInfoResolver { return a },
	)

	eventSink := &v1core.EventSinkImpl{Interface: client.CoreV1().Events("")}
	bufferedConfig := NewDefaultWebhookBatchConfig()
	bufferedConfig.BufferSize = 1
	bufferedConfig.MaxBatchSize = 1 //no wait
	return &Config{
		AuditSinkInformer:  informerFactory.Auditregistration().V1alpha1().AuditSinks(),
		AuditClassInformer: informerFactory.Auditregistration().V1alpha1().AuditClasses(),
		EventConfig:        EventConfig{Sink: eventSink},
		BufferedConfig:     bufferedConfig,
		WebhookConfig: WebhookConfig{
			AuthInfoResolverWrapper: authWrapper,
			ServiceResolver:         webhook.NewDefaultServiceResolver(),
		},
		WorkersCount: 2,
	}
}

// newTestBackend creates a test `backend` from `config`. It singlas immediately that
// cache has synced relying that test objects have been populated already
func newTestBackend(config *Config) *backend {
	b, _ := NewBackend(config)
	controller := b.(*backend).auditSinkControler.(*defaultAuditSinkControl)
	controller.auditSinksSynced = alwaysReady
	controller.auditClassesSynced = alwaysReady
	return b.(*backend)
}

// createAndRunTestBackend produces a test `backend` with the provided (optional) preloaded runtime objects (AuditClass,AuditSink),
// reconciler function, classtack channel on which the call stack for instrumented methods will be reported adn a stop chanel to signal
// test run exit.
// It returns a `backend` started adn ready for tsts and a `watcher` that can be used to simulate on the apiserver (audit) resources.
func createAndRunTestBackend(t *testing.T, preloadedAuditObjects []runtime.Object, reconcilerF func(reconicler *testReconciler, key uidKey) error, callstackChannel chan string, stopCh chan struct{}) (*backend, map[string]*watch.FakeWatcher) {
	informerFactory, watchers, client := newTestInformerFactory(preloadedAuditObjects, "auditsinks", "auditclasses")
	config := configForInformerFactory(informerFactory, client)
	dynamicBackend := newTestBackend(config)

	if reconcilerF != nil {
		// Setup test reconciler that will be injected to test that it was invoked with the test case epxected sink.
		dynamicBackend.auditSinkControler.(*defaultAuditSinkControl).reconciler = &testReconciler{
			t:                t,
			controller:       dynamicBackend.auditSinkControler.(*defaultAuditSinkControl),
			callStackChannel: callstackChannel,
			testReconcileF:   reconcilerF,
		}
	}

	// Start the classes and sinks watchers and the workqueue, send a watch event,
	// and make sure it hits the test reconciler's reconcile method for the right sink.
	go informerFactory.Auditregistration().V1alpha1().AuditClasses().Informer().Run(stopCh)
	go informerFactory.Auditregistration().V1alpha1().AuditSinks().Informer().Run(stopCh)
	go wait.Until(dynamicBackend.auditSinkControler.(*defaultAuditSinkControl).runWorker, 10*time.Millisecond, stopCh)

	// start delegates for the sinks that the test cases expects to start with
	for _, sw := range preloadedAuditObjects {
		if sink, ok := sw.(*auditregv1alpha1.AuditSink); ok {
			dynamicBackend.auditSinkControler.(*defaultAuditSinkControl).delegateManager.registerAndRunNewDelegate(sink)
		}
	}

	return dynamicBackend, watchers
}

type expectedDelegate struct {
	uid  types.UID
	sink *auditregv1alpha1.AuditSink
}

func TestDynamicDelegatesSyncAndProcesEvents(t *testing.T) {
	eventList1 := &atomic.Value{}
	eventList1.Store(auditinternal.EventList{})
	eventList2 := &atomic.Value{}
	eventList2.Store(auditinternal.EventList{})

	// start test servers
	server1 := httptest.NewServer(buildTestHandler(t, eventList1, "test1"))
	defer server1.Close()
	server2 := httptest.NewServer(buildTestHandler(t, eventList2, "test2"))
	defer server2.Close()

	testPolicy := auditregv1alpha1.Policy{
		Level: auditregv1alpha1.LevelMetadata,
		Stages: []auditregv1alpha1.Stage{
			auditregv1alpha1.StageResponseStarted,
		},
	}
	testEvent := auditinternal.Event{
		Level:      auditinternal.LevelMetadata,
		Stage:      auditinternal.StageResponseStarted,
		Verb:       "get",
		RequestURI: "/test/path",
	}
	testConfig1 := newAuditSink("test1", &server1.URL, testPolicy)
	testConfig2 := newAuditSink("test2", &server2.URL, testPolicy)
	testConfig1Update := testConfig1.DeepCopy()
	testConfig1Update.Spec.Webhook.ClientConfig.URL = &server2.URL
	testConfig1UpdateMeta := testConfig1.DeepCopy()
	testConfig1UpdateMeta.Labels = map[string]string{"my": "label"}

	badURL := "http://badtest"
	badConfig := newAuditSink("bad", &badURL, testPolicy)

	testcases := []struct {
		descr               string
		startWith           []runtime.Object
		expectedCalls       []string
		expectedEventStores map[string]*atomic.Value
		expectedDelegates   []*expectedDelegate
		testActivity        func(watchers map[string]*watch.FakeWatcher)
		events              []*auditinternal.Event
	}{
		{
			descr:               "create single delegate and it receives events",
			expectedEventStores: map[string]*atomic.Value{testConfig1.GetName(): eventList1},
			expectedDelegates: []*expectedDelegate{
				&expectedDelegate{
					uid:  testConfig1.GetUID(),
					sink: testConfig1,
				},
			},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Add(testConfig1)
			},
			expectedCalls: []string{"reconcile", "unregisterAndStopDelegate", "registerAndRunNewDelegate"},
		},
		{
			descr:               "create multiple delegates and they all receive events",
			startWith:           []runtime.Object{testConfig1, testConfig2},
			expectedEventStores: map[string]*atomic.Value{testConfig1.GetName(): eventList1, testConfig2.GetName(): eventList2},
			expectedDelegates: []*expectedDelegate{
				&expectedDelegate{
					uid:  testConfig1.GetUID(),
					sink: testConfig1,
				},
				&expectedDelegate{
					uid:  testConfig2.GetUID(),
					sink: testConfig2,
				},
			},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Add(testConfig1)
			},
		},		
		{
			descr:               "create delegate update it and it receives events",
			startWith:           []runtime.Object{testConfig1},
			expectedEventStores: map[string]*atomic.Value{testConfig2.GetName(): eventList2},
			expectedDelegates: []*expectedDelegate{
				&expectedDelegate{
					uid:  testConfig1Update.GetUID(),
					sink: testConfig1Update,
				},
			},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Modify(testConfig1Update)
			},
			expectedCalls: []string{"reconcile", "unregisterAndStopDelegate", "registerAndRunNewDelegate"},
		},
		{
			descr:               "update delegate metadata has no effect and receives events",
			startWith:           []runtime.Object{testConfig1},
			expectedEventStores: map[string]*atomic.Value{testConfig1.GetName(): eventList1},
			expectedDelegates: []*expectedDelegate{
				&expectedDelegate{
					uid:  testConfig1.GetUID(),
					sink: testConfig1,
				},
			},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Modify(testConfig1UpdateMeta)
			},
			expectedCalls: []string{},
		},
		{
			descr:               "can delete faulty delegate with events already sent",
			startWith:           []runtime.Object{testConfig1, badConfig},
			expectedEventStores: map[string]*atomic.Value{testConfig1.GetName(): eventList1},
			expectedDelegates: []*expectedDelegate{
				&expectedDelegate{
					uid:  testConfig1.GetUID(),
					sink: testConfig1,
				},
			},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Delete(badConfig)
			},
			expectedCalls: []string{"reconcile", "unregisterAndStopDelegate"},
			events:        []*auditinternal.Event{&testEvent, &testEvent},
		},
		{
			descr:               "delete delegate and find events sent to the rest",
			startWith:           []runtime.Object{testConfig1, testConfig2},
			expectedEventStores: map[string]*atomic.Value{testConfig2.GetName(): eventList2},
			expectedDelegates: []*expectedDelegate{
				&expectedDelegate{
					uid:  testConfig2.GetUID(),
					sink: testConfig2,
				},
			},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Delete(testConfig1)
			},
			expectedCalls: []string{"reconcile", "unregisterAndStopDelegate"},
		},		
	}

	for _, tc := range testcases {
		t.Run(tc.descr, func(t *testing.T) {
			stopCh := make(chan struct{})
			defer close(stopCh)
			callstackChannel := make(chan string)
			defer close(callstackChannel)
			dynamicBackend, watchers := createAndRunTestBackend(t, tc.startWith, reconcileAndNotify, callstackChannel, stopCh)
			// inject a test delegateManager that will record calls ot its methods
			dynamicBackend.auditSinkControler.(*defaultAuditSinkControl).delegateManager = &testDelegateManager{
				original:   dynamicBackend.auditSinkControler.(*defaultAuditSinkControl).delegateManager,
				callbackCh: callstackChannel,
			}
			// init sink stores if this test case requires any
			for _, es := range tc.expectedEventStores {
				es.Store(auditinternal.EventList{})
			}

			// run the test case action if htis test case requires any
			if tc.testActivity != nil {
				tc.testActivity(watchers)
			}

			// if a call stack is expected to execute before we continue in this testcase, wait for it or a timeout
			if len(tc.expectedCalls) > 0 {
				err := waitForCallStack(tc.expectedCalls, callstackChannel, time.Duration(100*time.Millisecond))
				require.NoError(t, err)
			}

			// assess the delegates state
			delegates := dynamicBackend.GetDelegates()
			if len(tc.expectedDelegates) > 0 {
				require.Len(t, delegates, len(tc.expectedDelegates))
				for _, ed := range tc.expectedDelegates {
					require.Contains(t, delegates, ed.uid)
					require.Equal(t, ed.sink, delegates[ed.uid].configuration)
				}
			} else {
				assert.Len(t, delegates, 0)
			}

			// send event(s) to backend delegates
			if len(tc.events) == 0 {
				tc.events = []*auditinternal.Event{&testEvent}
			}
			dynamicBackend.ProcessEvents(tc.events...)

			// if this test cases expects events to be stored by sinks, poll in parallel
			// in each store for them until they appear or a timeout occurs
			var wg sync.WaitGroup
			wg.Add(len(tc.expectedEventStores))
			for sinkName, es := range tc.expectedEventStores {
				go func() {
					err := checkForEvent(es, testEvent)
					assert.NoErrorf(t, err, "unable to find events sent to sink %s", sinkName)
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}

// checkForEvent will poll to check for an audit event in an atomic event list
func checkForEvent(a *atomic.Value, evSent auditinternal.Event) error {
	return wait.Poll(100*time.Millisecond, time.Second, func() (bool, error) {
		el := a.Load().(auditinternal.EventList)
		if len(el.Items) != 1 {
			return false, nil
		}
		evFound := el.Items[0]
		eq := reflect.DeepEqual(evSent, evFound)
		if !eq {
			return false, fmt.Errorf("event mismatch -- sent: %+v found: %+v", evSent, evFound)
		}
		return true, nil
	})
}

// buildTestHandler returns a handler that will update the atomic value passed in
// with the event list it receives
func buildTestHandler(t *testing.T, a *atomic.Value, sinkName string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("sink webook %s could not read request body: %v", sinkName, err)
		}
		el := auditinternal.EventList{}
		decoder := audit.Codecs.UniversalDecoder(auditv1.SchemeGroupVersion)
		if err := runtime.DecodeInto(decoder, body, &el); err != nil {
			t.Fatalf("sink webook %s failed decoding buf: %b, apiVersion: %s", sinkName, body, auditv1.SchemeGroupVersion)
		}
		defer r.Body.Close()
		a.Store(el)
		w.WriteHeader(200)
	})
}

func reconcileAndNotify(tester *testReconciler, key uidKey) error {
	tester.callStackChannel <- "reconcile"
	err := tester.controller.reconcile(key)
	if err != nil {
		tester.t.Logf("%v", err)
	}
	return err
}

func TestNewBackend(t *testing.T) {
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, noResyncPeriodFunc())
	auditSinkInformer := informerFactory.Auditregistration().V1alpha1().AuditSinks()
	auditClassInformer := informerFactory.Auditregistration().V1alpha1().AuditClasses()

	eventSink := &v1core.EventSinkImpl{Interface: client.CoreV1().Events("")}

	authWrapper := webhook.AuthenticationInfoResolverWrapper(
		func(a webhook.AuthenticationInfoResolver) webhook.AuthenticationInfoResolver { return a },
	)
	serviceResolver := webhook.NewDefaultServiceResolver()

	tcs := map[string]*Config{
		"should be ok with complete config": &Config{
			AuditSinkInformer:  auditSinkInformer,
			AuditClassInformer: auditClassInformer,
			EventConfig:        EventConfig{Sink: eventSink},
			BufferedConfig:     NewDefaultWebhookBatchConfig(),
			WebhookConfig: WebhookConfig{
				AuthInfoResolverWrapper: authWrapper,
				ServiceResolver:         serviceResolver,
			},
			WorkersCount: 10,
		},
		"should be ok without config properties with defaults": &Config{
			AuditSinkInformer:  auditSinkInformer,
			AuditClassInformer: auditClassInformer,
			EventConfig:        EventConfig{Sink: eventSink},
			WebhookConfig: WebhookConfig{
				AuthInfoResolverWrapper: authWrapper,
				ServiceResolver:         serviceResolver,
			},
			WorkersCount: 10,
		},
	}

	for desc, cfg := range tcs {
		t.Run(desc, func(t *testing.T) {
			//save initial config state before defaults apply
			initialCfg := *cfg
			b, err := NewBackend(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, b)
			if _b, ok := b.(*backend); ok {
				// Assert default when no BufferedConfig provided
				if initialCfg.BufferedConfig == nil {
					assert.Equal(t, _b.config.BufferedConfig, NewDefaultWebhookBatchConfig())
				}
			}
		})
	}
}

func TestShutdown(t *testing.T) {
	stopCh := make(chan struct{})
	b, _ := createAndRunTestBackend(t, nil, nil, nil, stopCh)
	// if the stop signal is not propagated correctly the buffers will not
	// close down gracefully, and the shutdown method will hang causing
	// the test will timeout.
	timeoutChan := make(chan struct{})
	successChan := make(chan struct{})
	go func() {
		time.Sleep(1 * time.Second)
		timeoutChan <- struct{}{}
	}()
	go func() {
		close(stopCh)
		b.Shutdown()
		successChan <- struct{}{}
	}()
	for {
		select {
		case <-timeoutChan:
			t.Error("shutdown timed out")
			return
		case <-successChan:
			return
		}
	}
}

type testControllerRunner struct {
   callstackChannel chan string
}

func (r *testControllerRunner) run(stopCh <-chan struct{}) error{
	r.callstackChannel<-"run"
	return nil
}

func TestRun(t *testing.T) {
	informerFactory, _, client := newTestInformerFactory(nil, "auditsinks", "auditclasses")
	config := configForInformerFactory(informerFactory, client)
	dynamicBackend := newTestBackend(config)
	stopCh := make(chan struct{})
	defer close(stopCh)
	callstackChannel := make(chan string)
	defer close(callstackChannel)
	dynamicBackend.auditSinkControler = &testControllerRunner{
		callstackChannel: callstackChannel,
	}
	
	go dynamicBackend.Run(stopCh)

	select {
	case <-time.After(10*time.Millisecond):
		t.Error("run timed out")
		return
	case <-callstackChannel:
		return
	}
}

// createTestReconcile creates a testReconcile function that uses `expectedSink` for tests
// when `reconcile` is hit. This reconcile function does not delegate to the original `reconcile`.
func createTestReconcile(r *testReconciler, expectedSink *auditregv1alpha1.AuditSink) func(r *testReconciler, key uidKey) error {
	return func(r *testReconciler, key uidKey) error {
		empty := reflect.New(reflect.TypeOf(expectedSink)).Elem().Interface()
		sink, err := r.controller.auditSinks.Get(key.name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				if reflect.DeepEqual(expectedSink, empty) {
					r.callStackChannel <- "reconcile"
					return nil
				}
			}
			r.t.Errorf("Expected to find auditsink under key %v: %v", key, err)
		}
		if !apiequality.Semantic.DeepDerivative(sink, expectedSink) ||
			reflect.DeepEqual(expectedSink, empty) {
			r.t.Errorf("\nExpected %#v,\nbut got %#v", expectedSink, sink)
			r.callStackChannel <- "reconcile"
			return nil
		}
		r.callStackChannel <- "reconcile"
		return err
	}
}

func TestAuditSinkReconcile(t *testing.T) {
	var (
		webhookURL = "https://test.namespace.svc.cluster.local"
	)
	testAuditSink := newAuditSink("test", &webhookURL, testPolicy)
	testClass := newAuditClass("testClass", []auditregv1alpha1.RequestSelector{
		{
			Verbs: []string{"get"},
		},
	})
	bindSinkToClass(testAuditSink, "testClass", auditregv1alpha1.LevelMetadata, auditregv1alpha1.StageResponseComplete)
	testClass1 := newAuditClass("testClass1", []auditregv1alpha1.RequestSelector{
		{
			Verbs: []string{"patch"},
		},
	})
	testAuditSinkUpdated := *testAuditSink
	bindSinkToClass(&testAuditSinkUpdated, "testClass1", auditregv1alpha1.LevelMetadata, auditregv1alpha1.StageResponseComplete)
	rogueClass := newAuditClass("rogueClass", []auditregv1alpha1.RequestSelector{
		{
			Verbs: []string{"get"},
		},
	})
	classUpdate := *testClass
	classUpdate.Spec.RequestSelectors = append(classUpdate.Spec.RequestSelectors, auditregv1alpha1.RequestSelector{
		Verbs: []string{"patch"},
	})

	testcases := []struct {
		descr        string
		startWith    []runtime.Object
		expectedSink *auditregv1alpha1.AuditSink
		testActivity func(watchers map[string]*watch.FakeWatcher)
	}{
		{
			descr:        "add sink will trigger reconcile of that sink",
			startWith:    []runtime.Object{testClass},
			expectedSink: testAuditSink,
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Add(testAuditSink)
			},
		},
		{
			descr:        "update sink spec will trigger reconcile of that sink",
			startWith:    []runtime.Object{testClass, testClass1, testAuditSink},
			expectedSink: &testAuditSinkUpdated,
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Modify(&testAuditSinkUpdated)
			},
		},
		{
			descr:     "delete sink will trigger reconcile of that sink",
			startWith: []runtime.Object{testClass, testClass1, &testAuditSinkUpdated},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditsinks"].Delete(&testAuditSinkUpdated)
			},
		},
		{
			descr:     "add unreferenced class will not trigger reconcile of any sinks",
			startWith: []runtime.Object{},
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditclasses"].Add(rogueClass)
			},
		},
		{
			descr:        "add class will trigger reconcile of referencing sinks",
			startWith:    []runtime.Object{testAuditSink},
			expectedSink: testAuditSink,
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditclasses"].Add(testClass)
			},
		},
		{
			descr:        "update class spec will trigger reconcile of referencing sinks",
			startWith:    []runtime.Object{testClass, testAuditSink},
			expectedSink: &testAuditSinkUpdated,
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditclasses"].Modify(&classUpdate)
			},
		},
		{
			descr:        "delete class will trigger reconcile of referencing sinks",
			startWith:    []runtime.Object{testClass, testClass1, &testAuditSinkUpdated},
			expectedSink: &testAuditSinkUpdated,
			testActivity: func(watchers map[string]*watch.FakeWatcher) {
				watchers["auditclasses"].Delete(testClass1)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.descr, func(t *testing.T) {

			informerFactory, watchers, client := newTestInformerFactory(tc.startWith, "auditsinks", "auditclasses")
			config := configForInformerFactory(informerFactory, client)
			dynamicBackend := newTestBackend(config)

			callStackCh := make(chan string)
			defer close(callStackCh)
			// Setup test reconciler that will be injected to test that it was invoked with the test case epxected sink.
			reconciler := &testReconciler{
				t:                t,
				controller:       dynamicBackend.auditSinkControler.(*defaultAuditSinkControl),
				callStackChannel: callStackCh,
			}
			reconciler.testReconcileF = createTestReconcile(reconciler, tc.expectedSink)
			dynamicBackend.auditSinkControler.(*defaultAuditSinkControl).reconciler = reconciler

			// Start the classes and sinks watchers and the workqueue
			stopCh := make(chan struct{})
			defer close(stopCh)
			go informerFactory.Auditregistration().V1alpha1().AuditClasses().Informer().Run(stopCh)
			go informerFactory.Auditregistration().V1alpha1().AuditSinks().Informer().Run(stopCh)
			go wait.Until(dynamicBackend.auditSinkControler.(*defaultAuditSinkControl).runWorker, 10*time.Millisecond, stopCh)

			// send a watch event and make sure it hits the test
			// reconciler's reconcile method for the right sink.
			tc.testActivity(watchers)

			// wait for a signal from the test reconciler or timeout
			err := waitForCallStack([]string{"reconcile"}, callStackCh, time.Duration(100*time.Millisecond))
			if tc.expectedSink != nil {
				require.NoError(t, err)
			}
		})
	}
}
