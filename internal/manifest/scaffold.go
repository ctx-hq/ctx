package manifest

// ScaffoldWithVisibility creates a scaffold manifest with an explicit visibility.
func ScaffoldWithVisibility(pkgType PackageType, scope, name, visibility string) *Manifest {
	m := Scaffold(pkgType, scope, name)
	m.Visibility = visibility
	return m
}

// ScaffoldPrivate creates a scaffold manifest pre-configured for private use.
func ScaffoldPrivate(scope, name string) *Manifest {
	m := Scaffold(TypeSkill, scope, name)
	m.Visibility = "private"
	m.Mutable = true
	return m
}

// DefaultVisibility returns the default visibility for a package type.
func DefaultVisibility(pkgType PackageType) string {
	return "public"
}
