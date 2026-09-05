package doctor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func (d *Doctor) checkProfile(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "profile",
		Title: "Active Profile & Path Resolution",
	}

	paths := config.ResolveProfilePaths("")
	res.Details = append(res.Details, fmt.Sprintf("Active Profile: %s", paths.ActiveProfileName))
	res.Details = append(res.Details, fmt.Sprintf("Home Directory: %s", paths.HomeDir))
	res.Details = append(res.Details, fmt.Sprintf("Config File: %s", paths.ConfigFile))
	res.Details = append(res.Details, fmt.Sprintf("Database: %s", paths.DBFile))
	res.Details = append(res.Details, fmt.Sprintf("Soul File: %s", paths.SoulFile))
	res.Details = append(res.Details, fmt.Sprintf("Skills Directory: %s", paths.SkillsDir))
	res.Details = append(res.Details, fmt.Sprintf("Policy File: %s", paths.PolicyFile))

	// Verify profile directory exists if a named profile is active
	if !paths.IsDefault {
		info, err := os.Stat(paths.HomeDir)
		if os.IsNotExist(err) {
			res.Status = StatusFail
			res.Message = fmt.Sprintf("Active profile %q directory does not exist (%s)", paths.ActiveProfileName, paths.HomeDir)
			res.Duration = time.Since(start)
			return res
		} else if err != nil {
			res.Status = StatusFail
			res.Message = fmt.Sprintf("Error accessing profile directory %s: %v", paths.HomeDir, err)
			res.Duration = time.Since(start)
			return res
		}

		if !info.IsDir() {
			res.Status = StatusFail
			res.Message = fmt.Sprintf("Profile path %s is not a directory", paths.HomeDir)
			res.Duration = time.Since(start)
			return res
		}
	}

	// Verify config file permissions if file exists
	if info, err := os.Stat(paths.ConfigFile); err == nil {
		perm := info.Mode().Perm()
		if perm > 0o600 {
			res.Status = StatusWarn
			res.Message = fmt.Sprintf("Profile config file mode %04o is looser than recommended 0600", perm)
			res.Duration = time.Since(start)
			return res
		}
	}

	res.Status = StatusPass
	res.Message = fmt.Sprintf("Active profile %q is valid and paths are accessible", paths.ActiveProfileName)
	res.Duration = time.Since(start)
	return res
}
