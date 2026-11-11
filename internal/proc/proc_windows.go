//go:build windows

package proc

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func scan() Snapshot {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return Snapshot{At: now()}
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return Snapshot{At: now()}
	}

	procs := make([]Process, 0, 256)

	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if name != "" {
			item := Process{
				PID:    int(entry.ProcessID),
				Parent: int(entry.ParentProcessID),
				Name:   name,
			}

			if looksHelium(name) || chromiumName(name) {
				item.Exe = imagePath(entry.ProcessID)
			}

			procs = append(procs, item)
		}

		entry.Size = uint32(unsafe.Sizeof(entry))
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return Snapshot{At: now(), Procs: procs}
}

func imagePath(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))

	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}

	return windows.UTF16ToString(buffer[:size])
}
