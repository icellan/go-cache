/*
Package cache is a simple redis cache dependency system on-top of the famous redigo package
*/
package cache

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

// Package constants (commands)
const (
	addToSetCommand      string = "SADD"
	authCommand          string = "AUTH"
	deleteCommand        string = "DEL"
	dependencyPrefix     string = "depend:"
	evalCommand          string = "EVALSHA"
	executeCommand       string = "EXEC"
	existsCommand        string = "EXISTS"
	expireCommand        string = "EXPIRE"
	flushAllCommand      string = "FLUSHALL"
	getCommand           string = "GET"
	hashGetCommand       string = "HGET"
	hashKeySetCommand    string = "HSET"
	hashMapGetCommand    string = "HMGET"
	hashMapSetCommand    string = "HMSET"
	isMemberCommand      string = "SISMEMBER"
	keysCommand          string = "KEYS"
	listPushCommand      string = "RPUSH"
	listRangeCommand     string = "LRANGE"
	multiCommand         string = "MULTI"
	pingCommand          string = "PING"
	removeMemberCommand  string = "SREM"
	scriptCommand        string = "SCRIPT"
	selectCommand        string = "SELECT"
	setCommand           string = "SET"
	setExpirationCommand string = "SETEX"
)

// Get gets a key from redis
func Get(key string) (string, error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the command
	return redis.String(conn.Do(getCommand, key))
}

// GetBytes gets a key from redis in bytes
func GetBytes(key string) ([]byte, error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the command
	return redis.Bytes(conn.Do(getCommand, key))
}

// GetList returns a []string stored in redis list
func GetList(key string) (list []string, err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// This command takes two parameters specifying the range: 0 start, -1 is the end of the list
	var values []interface{}
	if values, err = redis.Values(conn.Do(listRangeCommand, key, 0, -1)); err != nil {
		return
	}

	// Scan slice by value, return with destination
	err = redis.ScanSlice(values, &list)
	return
}

// SetList saves a slice as a redis list (appends)
func SetList(key string, slice []string) (err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Create the arguments
	args := make([]interface{}, len(slice)+1)
	args[0] = key

	// Loop members
	for i, param := range slice {
		args[i+1] = param
	}

	// Fire the set command
	_, err = conn.Do(listPushCommand, args...)
	return
}

// GetAllKeys returns a []string of keys
func GetAllKeys() (keys []string, err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Get all the keys
	return redis.Strings(conn.Do(keysCommand, "*"))
}

// Set will set the key in redis and keep a reference to each dependency
// value can be both a string or []byte
func Set(key string, value interface{}, dependencies ...string) (err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the set command
	if _, err = conn.Do(setCommand, key, value); err != nil {
		return
	}

	// Link and return the error
	return linkDependencies(conn, key, dependencies...)
}

// SetExp will set the key in redis and keep a reference to each dependency
// value can be both a string or []byte
func SetExp(key string, value interface{}, ttl time.Duration, dependencies ...string) error {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the set expiration
	if _, err := conn.Do(setExpirationCommand, key, int64(ttl.Seconds()), value); err != nil {
		return err
	}

	// Link and return the error
	return linkDependencies(conn, key, dependencies...)
}

// Exists checks if a key is present or not
func Exists(key string) (bool, error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the command
	return redis.Bool(conn.Do(existsCommand, key))
}

// Expire sets the expiration for a given key
func Expire(key string, duration time.Duration) (err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the expire command
	_, err = conn.Do(expireCommand, key, int64(duration.Seconds()))
	return
}

// Delete is an alias for KillByDependency()
func Delete(keys ...string) (total int, err error) {
	return KillByDependency(keys...)
}

// DeleteWithoutDependency will remove keys without using dependency script
func DeleteWithoutDependency(keys ...string) (total int, err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Loop all keys and delete
	for _, key := range keys {
		if _, err = conn.Do(deleteCommand, key); err != nil {
			return
		}
		total++
	}

	return
}

// HashSet will set the hashKey to the value in the specified hashName and link a
// reference to each dependency for the entire hash
func HashSet(hashName, hashKey string, value interface{}, dependencies ...string) error {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Set the hash key
	if _, err := conn.Do(hashKeySetCommand, hashName, hashKey, value); err != nil {
		return err
	}

	// Link and return the error
	return linkDependencies(conn, hashName, dependencies...)
}

// HashGet gets a key from redis via hash
func HashGet(hash, key string) (string, error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the command
	return redis.String(conn.Do(hashGetCommand, hash, key))
}

// HashMapGet gets values from a hash map for corresponding keys
func HashMapGet(hashName string, keys ...interface{}) ([]string, error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Build up the arguments
	keys = append([]interface{}{hashName}, keys...)

	// Fire the command with all keys
	return redis.Strings(conn.Do(hashMapGetCommand, keys...))
}

// HashMapSet will set the hashKey to the value in the specified hashName and link a
// reference to each dependency for the entire hash
func HashMapSet(hashName string, pairs [][2]interface{}, dependencies ...string) error {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Set the arguments
	args := make([]interface{}, 0, 2*len(pairs)+1)
	args = append(args, hashName)
	for _, pair := range pairs {
		args = append(args, pair[0])
		args = append(args, pair[1])
	}

	// Set the hash map
	if _, err := conn.Do(hashMapSetCommand, args...); err != nil {
		return err
	}

	// Link and return the error
	return linkDependencies(conn, hashName, dependencies...)
}

// HashMapSetExp will set the hashKey to the value in the specified hashName and link a
// reference to each dependency for the entire hash
func HashMapSetExp(hashName string, pairs [][2]interface{}, ttl time.Duration, dependencies ...string) error {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Set the arguments
	args := make([]interface{}, 0, 2*len(pairs)+1)
	args = append(args, hashName)
	for _, pair := range pairs {
		args = append(args, pair[0])
		args = append(args, pair[1])
	}

	// Set the hash map
	if _, err := conn.Do(hashMapSetCommand, args...); err != nil {
		return err
	}

	// Fire the expire command
	if _, err := conn.Do(expireCommand, hashName, ttl.Seconds()); err != nil {
		return err
	}

	// Link and return the error
	return linkDependencies(conn, hashName, dependencies...)
}

// SetAdd will add the member to the Set and link a reference to each dependency for the entire Set
func SetAdd(setName, member interface{}, dependencies ...string) error {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Add member to set
	if _, err := conn.Do(addToSetCommand, setName, member); err != nil {
		return err
	}

	// Link and return the error
	return linkDependencies(conn, setName, dependencies...)
}

// SetAddMany will add many values to an existing set
func SetAddMany(setName string, members ...interface{}) (err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Create the arguments
	args := make([]interface{}, len(members)+1)
	args[0] = setName

	// Loop members
	for i, key := range members {
		args[i+1] = key
	}

	// Fire the delete
	_, err = conn.Do(addToSetCommand, args...)
	return

	// Link and return the error //todo: add dependencies back?
	// return linkDependencies(conn, setName, dependencies...)
}

// SetIsMember returns if the member is part of the set
func SetIsMember(set, member interface{}) (bool, error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Check if is member
	return redis.Bool(conn.Do(isMemberCommand, set, member))
}

// SetRemoveMember removes the member from the set
func SetRemoveMember(set, member interface{}) (err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Remove and return
	_, err = conn.Do(removeMemberCommand, set, member)
	return
}

// DestroyCache will flush the entire redis server. It only removes keys, not scripts
func DestroyCache() (err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Fire the command
	_, err = conn.Do(flushAllCommand)
	return
}

// KillByDependency removes all keys which are listed as depending on the key(s)
// Also: Delete()
func KillByDependency(keys ...string) (total int, err error) {

	// Get a connection and defer closing the connection
	conn := GetConnection()
	defer func() {
		_ = conn.Close()
	}()

	// Do we have keys to kill?
	if len(keys) == 0 {
		return
	}

	// Create the arguments
	args := make([]interface{}, len(keys)+2)
	deleteArgs := make([]interface{}, len(keys))

	args[0] = killByDependencySha
	args[1] = 0

	// Loop keys
	for i, key := range keys {
		args[i+2] = dependencyPrefix + key
		deleteArgs[i] = key
	}

	// Create the script
	if total, err = redis.Int(conn.Do(evalCommand, args...)); err != nil {
		return
	}

	// Fire the delete
	_, err = conn.Do(deleteCommand, deleteArgs...)
	return
}

// linkDependencies links any dependencies
func linkDependencies(conn redis.Conn, key interface{}, dependencies ...string) (err error) {

	// No dependencies given
	if len(dependencies) == 0 {
		return
	}

	// Send the multi command
	if err = conn.Send(multiCommand); err != nil {
		return
	}

	// Add all to the set
	for _, dependency := range dependencies {
		if err = conn.Send(addToSetCommand, dependencyPrefix+dependency, key); err != nil {
			return
		}
	}

	// Fire the exec command
	_, err = redis.Values(conn.Do(executeCommand))
	return
}
