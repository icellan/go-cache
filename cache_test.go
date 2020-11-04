package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rafaeljusto/redigomock"
	"github.com/stretchr/testify/assert"
)

// Testing variables
const (
	// testKillDependencyHash   = "a648f768f57e73e2497ccaa113d5ad9e731c5cd8"
	testDependantKey         = "test-dependant-key-name"
	testHashName             = "test-hash-name"
	testIdleTimeout          = 240
	testKey                  = "test-key-name"
	testLocalConnectionURL   = "redis://localhost:6379"
	testMaxActiveConnections = 0
	testMaxConnLifetime      = 0
	testMaxIdleConnections   = 10
	testStringValue          = "test-string-value"
)

// loadMockRedis will load a mocked redis connection
func loadMockRedis() (conn *redigomock.Conn, pool *redis.Pool) {
	conn = redigomock.NewConn()
	pool = &redis.Pool{
		Dial:            func() (redis.Conn, error) { return conn, nil },
		IdleTimeout:     time.Duration(testIdleTimeout) * time.Second,
		MaxActive:       testMaxActiveConnections,
		MaxConnLifetime: testMaxConnLifetime,
		MaxIdle:         testMaxIdleConnections,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, doErr := c.Do(pingCommand)
			return doErr
		},
	}
	return
}

// loadRealRedis will load a real redis connection
func loadRealRedis() (conn redis.Conn, pool *redis.Pool, err error) {
	pool, err = Connect(
		testLocalConnectionURL,
		testMaxActiveConnections,
		testMaxIdleConnections,
		testMaxConnLifetime,
		testIdleTimeout,
		true,
	)
	if err != nil {
		return
	}

	conn = GetConnection(pool)
	return
}

// clearRealRedis will clear a real redis db
func clearRealRedis(conn redis.Conn) error {
	return DestroyCache(conn)
}

// endTest end tests the same way
func endTest(pool *redis.Pool, conn redis.Conn) {
	CloseAll(pool, conn)
}

/*// startTest start all tests the same way
func startTestCustom() (pool *redis.Pool, err error) {
	return Connect(
		testLocalConnectionURL,
		testMaxActiveConnections,
		testMaxIdleConnections,
		testMaxConnLifetime,
		testIdleTimeout,
		true,
		redis.DialKeepAlive(10*time.Second),
	)
}*/

// TestSet is testing the method Set()
func TestSet(t *testing.T) {

	t.Run("set command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase     string
			key          string
			value        string
			dependencies []string
		}{
			{"key with dependencies", testKey, testStringValue, []string{testDependantKey}},
			{"key with no dependencies", testKey, testStringValue, []string{}},
			{"key with empty value", testKey, "", []string{}},
			{"key with spaces", "key name", "some val", []string{}},
			{"key with symbols", ".key name;!()\\", "", []string{}},
			{"key with symbols and value as symbols", ".key name;!()\\", `\ / ; [ ] { }!`, []string{}},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				var commands []*redigomock.Cmd

				// The main command to test
				commands = append(commands, conn.Command(setCommand, test.key, test.value).Expect(test.value))

				// Loop for each dependency
				if len(test.dependencies) > 0 {
					commands = append(commands, conn.Command(multiCommand))
					for _, dep := range test.dependencies {
						commands = append(commands, conn.Command(addToSetCommand, dependencyPrefix+dep, test.key))
					}
					commands = append(commands, conn.Command(executeCommand))

					err := Set(conn, test.key, test.value, test.dependencies...)
					assert.NoError(t, err)
				} else {
					err := Set(conn, test.key, test.value, test.dependencies...)
					assert.NoError(t, err)
				}

				for _, c := range commands {
					assert.Equal(t, true, c.Called)
				}
			})
		}
	})

	t.Run("set command using real redis", func(t *testing.T) {

		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase     string
			key          string
			value        string
			dependencies []string
		}{
			{"key with dependencies", testKey, testStringValue, []string{testDependantKey}},
			{"key with no dependencies", testKey, testStringValue, []string{""}},
			{"key with empty value", testKey, "", []string{""}},
			{"key with spaces", "key name", "some val", []string{""}},
			{"key with symbols", ".key name;!()\\", "", []string{""}},
			{"key with symbols and value as symbols", ".key name;!()\\", `\ / ; [ ] { }!`, []string{""}},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {

				// Start with a fresh db
				err = clearRealRedis(conn)
				assert.NoError(t, err)

				// Run command
				err = Set(conn, test.key, test.value, test.dependencies...)
				assert.NoError(t, err)

				// Validate via getting the data from redis
				var testVal string
				testVal, err = Get(conn, test.key)
				assert.NoError(t, err)
				assert.Equal(t, test.value, testVal)
			})
		}
	})
}

// ExampleSet is an example of the method Set()
func ExampleSet() {

	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = Set(conn, testKey, testStringValue, testDependantKey)
	fmt.Printf("set: %s value: %s dep key: %s", testKey, testStringValue, testDependantKey)
	// Output:set: test-key-name value: test-string-value dep key: test-dependant-key-name
}

// TestSetExp is testing the method SetExp()
func TestSetExp(t *testing.T) {

	t.Run("set exp command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase     string
			key          string
			value        string
			expiration   time.Duration
			dependencies []string
		}{
			{"key with dependencies", "test-set-exp", testStringValue, 2 * time.Second, []string{testDependantKey}},
			{"key with no dependencies", "test-set2", testStringValue, 2 * time.Second, []string{}},
			{"key with empty value", "test-set3", "", 2 * time.Second, []string{}},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				var commands []*redigomock.Cmd

				// The main command to test
				commands = append(commands, conn.Command(setExpirationCommand, test.key, int64(test.expiration.Seconds()), test.value).Expect(test.value))

				// Loop for each dependency
				if len(test.dependencies) > 0 {
					commands = append(commands, conn.Command(multiCommand))
					for _, dep := range test.dependencies {
						commands = append(commands, conn.Command(addToSetCommand, dependencyPrefix+dep, test.key))
					}
					commands = append(commands, conn.Command(executeCommand))

					err := SetExp(conn, test.key, test.value, test.expiration, test.dependencies...)
					assert.NoError(t, err)
				} else {
					err := SetExp(conn, test.key, test.value, test.expiration, test.dependencies...)
					assert.NoError(t, err)
				}

				for _, c := range commands {
					assert.Equal(t, true, c.Called)
				}
			})
		}
	})

	t.Run("set exp command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = SetExp(conn, testKey, testStringValue, 2*time.Second, testDependantKey)
		assert.NoError(t, err)

		// Check that the command worked
		var testVal string
		testVal, err = Get(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, testStringValue, testVal)

		// Wait a few seconds and test
		t.Log("sleeping for 3 seconds...")
		time.Sleep(time.Second * 3)

		// Check that the key is expired
		testVal, err = Get(conn, testKey)
		assert.Error(t, err)
		assert.Equal(t, "", testVal)
		assert.Equal(t, redis.ErrNil, err)
	})
}

// ExampleSetExp is an example of the method SetExp()
func ExampleSetExp() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = SetExp(conn, testKey, testStringValue, 2*time.Minute, testDependantKey)
	fmt.Printf("set: %s value: %s exp: %v dep key: %s", testKey, testStringValue, 2*time.Minute, testDependantKey)
	// Output:set: test-key-name value: test-string-value exp: 2m0s dep key: test-dependant-key-name
}

// TestGet is testing the method Get()
func TestGet(t *testing.T) {

	t.Run("get command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase string
			key      string
			value    interface{}
		}{
			{"valid value", testHashName, testStringValue},
			{"new key", "test-hash-name1", testStringValue},
			{"third key", "test-hash-name2", testStringValue},
			{"fourth key", "test-hash-name3", ""},
			{"no name", "", ""},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				// The main command to test
				getCmd := conn.Command(getCommand, test.key).Expect(test.value)

				val, err := Get(conn, test.key)
				assert.NoError(t, err)
				assert.Equal(t, true, getCmd.Called)
				assert.Equal(t, test.value, val)
			})
		}
	})

	t.Run("get command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = Set(conn, testKey, testStringValue, testDependantKey)
		assert.NoError(t, err)

		// Check that the command worked
		var testVal string
		testVal, err = Get(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, testStringValue, testVal)
	})
}

// ExampleGet is an example of the method Get()
func ExampleGet() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = Set(conn, testKey, testStringValue, testDependantKey)

	// Get the value
	_, _ = Get(conn, testKey)
	fmt.Printf("got value: %s", testStringValue)
	// Output:got value: test-string-value
}

// TestGetBytes is testing the method GetBytes()
func TestGetBytes(t *testing.T) {

	t.Run("get bytes command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase string
			key      string
			value    string
		}{
			{"valid value", testHashName, testStringValue},
			{"new key", "test-hash-name1", testStringValue},
			{"third key", "test-hash-name2", testStringValue},
			{"fourth key", "test-hash-name3", ""},
			{"no name", "", ""},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				// The main command to test
				getCmd := conn.Command(getCommand, test.key).Expect([]byte(test.value))

				val, err := GetBytes(conn, test.key)
				assert.NoError(t, err)
				assert.Equal(t, true, getCmd.Called)
				assert.Equal(t, []byte(test.value), val)
			})
		}
	})

	t.Run("get bytes command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = Set(conn, testKey, testStringValue, testDependantKey)
		assert.NoError(t, err)

		// Check that the command worked
		var testVal []byte
		testVal, err = GetBytes(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, []byte(testStringValue), testVal)
	})
}

// ExampleGetBytes is an example of the method GetBytes()
func ExampleGetBytes() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = Set(conn, testKey, testStringValue, testDependantKey)

	// Get the value
	_, _ = GetBytes(conn, testKey)
	fmt.Printf("got value: %s", testStringValue)
	// Output:got value: test-string-value
}

// TestGetAllKeys is testing the method GetAllKeys()
func TestGetAllKeys(t *testing.T) {

	t.Run("get all keys command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		conn.Clear()

		// The main command to test
		getCmd := conn.Command(keysCommand, allKeysCommand).Expect([]interface{}{[]byte(testKey)})

		val, err := GetAllKeys(conn)
		assert.NoError(t, err)
		assert.Equal(t, true, getCmd.Called)
		assert.Equal(t, []string{testKey}, val)
	})

	t.Run("get all keys command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = Set(conn, testKey, testStringValue, testDependantKey)
		assert.NoError(t, err)

		// Check that the command worked
		var keys []string
		keys, err = GetAllKeys(conn)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(keys))
	})
}

// ExampleGetAllKeys is an example of the method GetAllKeys()
func ExampleGetAllKeys() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = Set(conn, testKey, testStringValue, testDependantKey)

	// Get the keys
	_, _ = GetAllKeys(conn)
	fmt.Printf("found keys: %d", len([]string{testKey, testDependantKey}))
	// Output:found keys: 2
}

// TestExists is testing the method Exists()
func TestExists(t *testing.T) {

	t.Run("exists command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		conn.Clear()

		// todo: add table tests

		// The main command to test
		existsCmd := conn.Command(existsCommand, testKey).Expect(interface{}(int64(1)))

		val, err := Exists(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, true, existsCmd.Called)
		assert.Equal(t, true, val)
	})

	t.Run("exists command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = Set(conn, testKey, testStringValue, testDependantKey)
		assert.NoError(t, err)

		// Check that the command worked
		var found bool
		found, err = Exists(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, true, found)
	})
}

// ExampleExists is an example of the method Exists()
func ExampleExists() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = Set(conn, testKey, testStringValue, testDependantKey)

	// Get the value
	_, _ = Exists(conn, testKey)
	fmt.Print("key exists")
	// Output:key exists
}

// TestExpire is testing the method Expire()
func TestExpire(t *testing.T) {

	t.Run("expire command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase   string
			key        string
			expiration time.Duration
		}{
			{"regular key", "test-set-exp", 2 * time.Second},
			{"lots of time", "test-set2", 200 * time.Hour},
			{"no time", "test-set3", 0},
			{"no key name", "", 2 * time.Second},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				// The main command to test
				expireCmd := conn.Command(expireCommand, test.key, int64(test.expiration.Seconds()))

				err := Expire(conn, test.key, test.expiration)
				assert.NoError(t, err)
				assert.Equal(t, true, expireCmd.Called)
			})
		}
	})

	t.Run("expire command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = SetExp(conn, testKey, testStringValue, 5*time.Second, testDependantKey)
		assert.NoError(t, err)

		// Check that the command worked
		var testVal string
		testVal, err = Get(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, testStringValue, testVal)

		// Expire
		err = Expire(conn, testKey, 1*time.Second)
		if err != nil {
			t.Fatal(err.Error())
		}

		// Wait a few seconds and test
		t.Log("sleeping for 2 seconds...")
		time.Sleep(time.Second * 2)

		// Check that the key is expired
		testVal, err = Get(conn, testKey)
		assert.Error(t, err)
		assert.Equal(t, redis.ErrNil, err)
		assert.Equal(t, "", testVal)
	})
}

// ExampleExpire is an example of the method Expire()
func ExampleExpire() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = Set(conn, testKey, testStringValue, testDependantKey)

	// Fire the command
	_ = Expire(conn, testKey, 1*time.Minute)
	fmt.Printf("expiration on key: %s set for: %v", testKey, 1*time.Minute)
	// Output:expiration on key: test-key-name set for: 1m0s
}

// TestDestroyCache is testing the method DestroyCache()
func TestDestroyCache(t *testing.T) {

	t.Run("destroy cache / flush all command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		conn.Clear()

		// The main command to test
		destroyCmd := conn.Command(flushAllCommand)

		err := DestroyCache(conn)
		assert.NoError(t, err)
		assert.Equal(t, true, destroyCmd.Called)
	})

	t.Run("destroy cache / flush all command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = Set(conn, testKey, testStringValue, testDependantKey)
		assert.NoError(t, err)

		// Test getting a value
		var val string
		val, err = Get(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, val, testStringValue)

		// Check that the command worked
		err = DestroyCache(conn)
		assert.NoError(t, err)

		// Value should not exist
		val, err = Get(conn, testKey)
		assert.Error(t, err)
		assert.Equal(t, err, redis.ErrNil)
		assert.Equal(t, val, "")
	})
}

// ExampleDestroyCache is an example of the method DestroyCache()
func ExampleDestroyCache() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Fire the command
	_ = DestroyCache(conn)
	fmt.Print("cache destroyed")
	// Output:cache destroyed
}

// TestGetList test the method GetList()
func TestGetList(t *testing.T) {

	t.Run("get list command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase           string
			key                string
			inputList          []string
			expectedList       []interface{}
			expectedStringList []string
		}{
			{
				"empty list",
				"test-set",
				[]string{""},
				[]interface{}{""},
				[]string{""},
			},
			{
				"one item",
				"test-set",
				[]string{"1"},
				[]interface{}{[]byte("1")},
				[]string{"1"},
			},
			{
				"multiple items",
				"test-set",
				[]string{"1", "1"},
				[]interface{}{[]byte("1"), []byte("1")},
				[]string{"1", "1"},
			},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				// The main command to test
				getCmd := conn.Command(listRangeCommand, test.key, 0, -1).Expect(test.expectedList)

				list, err := GetList(conn, test.key)
				assert.NoError(t, err)
				assert.Equal(t, true, getCmd.Called)
				assert.Equal(t, test.expectedStringList, list)
			})
		}
	})

	t.Run("get list command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = SetList(conn, testKey, []string{testStringValue})
		assert.NoError(t, err)

		// Check that the command worked
		var list []string
		list, err = GetList(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, []string{testStringValue}, list)
	})
}

// ExampleGetList is an example of the method GetList()
func ExampleGetList() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = SetList(conn, testKey, []string{testStringValue})

	// Fire the command
	_, _ = GetList(conn, testKey)
	fmt.Printf("got list: %v", []string{testStringValue})
	// Output:got list: [test-string-value]
}

// TestSetList test the method SetList()
func TestSetList(t *testing.T) {

	t.Run("set list command using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase  string
			key       string
			inputList []string
		}{
			{
				"empty list",
				"test-set",
				[]string{""},
			},
			{
				"one item",
				"test-set",
				[]string{"1"},
			},
			{
				"multiple items",
				"test-set",
				[]string{"1", "1"},
			},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				// Create the arguments
				args := make([]interface{}, len(test.inputList)+1)
				args[0] = test.key

				// Loop members
				for i, param := range test.inputList {
					args[i+1] = param
				}

				// The main command to test
				setCmd := conn.Command(listPushCommand, args...)

				err := SetList(conn, test.key, test.inputList)
				assert.NoError(t, err)
				assert.Equal(t, true, setCmd.Called)
			})
		}
	})

	t.Run("set list command using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Fire the command
		err = SetList(conn, testKey, []string{testStringValue})
		assert.NoError(t, err)

		// Check that the command worked
		var list []string
		list, err = GetList(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, []string{testStringValue}, list)
	})
}

// ExampleSetList is an example of the method SetList()
func ExampleSetList() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = SetList(conn, testKey, []string{testStringValue})

	// Fire the command
	_, _ = GetList(conn, testKey)
	fmt.Printf("got list: %v", []string{testStringValue})
	// Output:got list: [test-string-value]
}

// TestDeleteWithoutDependency test the method DeleteWithoutDependency()
func TestDeleteWithoutDependency(t *testing.T) {

	t.Run("delete without using dependencies using mocked redis", func(t *testing.T) {
		t.Parallel()

		// Load redis
		conn, pool := loadMockRedis()
		assert.NotNil(t, pool)
		defer endTest(pool, conn)

		var tests = []struct {
			testCase     string
			keys         []string
			totalDeleted int
		}{
			{
				"empty list",
				[]string{},
				0,
			},
			{
				"one item",
				[]string{testKey},
				1,
			},
			{
				"multiple items",
				[]string{testKey, testKey + "2"},
				2,
			},
		}
		for _, test := range tests {
			t.Run(test.testCase, func(t *testing.T) {
				conn.Clear()

				// The main command to test
				var commands []*redigomock.Cmd
				for _, key := range test.keys {
					cmd := conn.Command(deleteCommand, key)
					commands = append(commands, cmd)
				}

				total, err := DeleteWithoutDependency(conn, test.keys...)
				assert.NoError(t, err)
				assert.Equal(t, test.totalDeleted, total)
				for _, c := range commands {
					assert.Equal(t, true, c.Called)
				}
			})
		}
	})

	t.Run("delete without using dependencies using real redis", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping live local redis tests")
		}

		// Load redis
		conn, pool, err := loadRealRedis()
		assert.NotNil(t, pool)
		assert.NoError(t, err)
		defer endTest(pool, conn)

		// Start with a fresh db
		err = clearRealRedis(conn)
		assert.NoError(t, err)

		// Set a key
		err = Set(conn, testKey, testStringValue, testDependantKey)
		assert.NoError(t, err)

		// Fire the command
		var total int
		total, err = DeleteWithoutDependency(conn, testKey)
		assert.NoError(t, err)
		assert.Equal(t, 1, total)

		// Check that the command worked
		var val string
		val, err = Get(conn, testKey)
		assert.Error(t, err)
		assert.Equal(t, redis.ErrNil, err)
		assert.Equal(t, "", val)
	})
}

// ExampleDeleteWithoutDependency is an example of the method DeleteWithoutDependency()
func ExampleDeleteWithoutDependency() {
	// Load a mocked redis for testing/examples
	conn, pool := loadMockRedis()

	// Close connections at end of request
	defer CloseAll(pool, conn)

	// Set the key/value
	_ = Set(conn, testKey, testStringValue)
	_ = Set(conn, testKey+"2", testStringValue)

	// Delete keys
	_, _ = DeleteWithoutDependency(conn, testKey, testKey+"2")
	fmt.Printf("deleted keys: %d", 2)
	// Output:deleted keys: 2
}
