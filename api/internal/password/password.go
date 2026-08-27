package password

import (
	"sync"

	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

const dummyPassword = "dummy-password"

// A fixed valid hash at DefaultCost, used only if hashing dummyPassword fails.
// It is not that password's hash — nothing may rely on the two matching.
const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// MinCost is the cheapest hashing the algorithm accepts, for the test suite.
const MinCost = bcrypt.MinCost

var (
	dummyMutex  sync.Mutex
	dummyHashes = map[int]string{}
)

// Cost reports the cost new hashes use; out-of-range values fall back to the
// default rather than failing at hash time. Lowering it weakens every password
// hashed while it is in effect — it exists for the test suite.
func Cost() int {
	cost := viper.GetInt("BCRYPT_COST")
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return bcrypt.DefaultCost
	}
	return cost
}

// Hash hashes plain at the configured cost.
func Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), Cost())
	return string(hash), err
}

// Compare reports whether plain is the password behind hash, returning nil on a
// match. The cost is read from the hash, so hashes outlive a cost change.
func Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

// Dummy returns a hash to compare an unknown account's login against, so a
// failed login costs the same work whether or not the account exists. It must
// carry the configured cost, or that timing leak reopens wherever it is lowered.
func Dummy() string {
	cost := Cost()

	dummyMutex.Lock()
	defer dummyMutex.Unlock()

	if hash, ok := dummyHashes[cost]; ok {
		return hash
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(dummyPassword), cost)
	if err != nil {
		return dummyPasswordHash
	}
	dummyHashes[cost] = string(hash)
	return string(hash)
}
