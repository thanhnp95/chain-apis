package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// Parse parses a semantic version string
func Parse(version string) (*Version, error) {
	matches := semverRegex.FindStringSubmatch(version)
	if matches == nil {
		return nil, fmt.Errorf("invalid semantic version: %s", version)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Build:      matches[5],
	}, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions
// Returns -1 if v < other, 0 if v == other, 1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Handle prerelease comparison
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1 // No prerelease > prerelease
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1 // Prerelease < no prerelease
	}
	if v.Prerelease != other.Prerelease {
		return strings.Compare(v.Prerelease, other.Prerelease)
	}

	return 0
}

// LessThan returns true if v < other
func (v *Version) LessThan(other *Version) bool {
	return v.Compare(other) < 0
}

// GreaterThan returns true if v > other
func (v *Version) GreaterThan(other *Version) bool {
	return v.Compare(other) > 0
}

// Equal returns true if v == other
func (v *Version) Equal(other *Version) bool {
	return v.Compare(other) == 0
}

// GreaterThanOrEqual returns true if v >= other
func (v *Version) GreaterThanOrEqual(other *Version) bool {
	return v.Compare(other) >= 0
}

// LessThanOrEqual returns true if v <= other
func (v *Version) LessThanOrEqual(other *Version) bool {
	return v.Compare(other) <= 0
}

// Semver is a simple semver type compatible with btcd version info
type Semver struct {
	Major uint32
	Minor uint32
	Patch uint32
}

// NewSemver creates a new Semver
func NewSemver(major, minor, patch uint32) Semver {
	return Semver{Major: major, Minor: minor, Patch: patch}
}

// String returns the string representation
func (s Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
}

// AnyCompatible checks if nodeVer is compatible with any of the given versions
// Compatibility is based on major version only (semver rules)
func AnyCompatible(compatible []Semver, nodeVer Semver) bool {
	for _, v := range compatible {
		if v.Major == nodeVer.Major {
			return true
		}
	}
	return false
}
