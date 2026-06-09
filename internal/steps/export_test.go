package steps

type RegistrySnapshot = []entry

func TakeSnapshot() RegistrySnapshot { return snapshot() }

func RestoreSnapshot(s RegistrySnapshot) { reset(s) }
