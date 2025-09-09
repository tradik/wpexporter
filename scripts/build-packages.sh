#!/bin/bash

# WordPress Export JSON - Package Builder Script
# Builds DEB, RPM, and TGZ packages for distribution

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
APP_NAME="wpexportjson"
XMLRPC_NAME="wpxmlrpc"
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")}
BUILD_DIR="build"
DIST_DIR="dist"
PKG_DIR="$DIST_DIR/packages"
MAINTAINER="WordPress Export JSON Team <info@example.com>"
DESCRIPTION="WordPress content export tool with REST API and XML-RPC support"
HOMEPAGE="https://github.com/tradik/wpexporter"

# Create package directories
mkdir -p "$PKG_DIR"/{deb,rpm,tgz}

echo -e "${BLUE}Building packages for $APP_NAME $VERSION${NC}"

# Function to create DEB package
build_deb() {
    local arch=$1
    local binary_arch=$2
    
    echo -e "${YELLOW}Building DEB package for $arch...${NC}"
    
    local deb_dir="$PKG_DIR/deb/${APP_NAME}_${VERSION}_${arch}"
    mkdir -p "$deb_dir"/{DEBIAN,usr/bin,usr/share/doc/$APP_NAME,etc/$APP_NAME}
    
    # Copy binaries
    cp "$DIST_DIR/${APP_NAME}-linux-${binary_arch}" "$deb_dir/usr/bin/$APP_NAME"
    cp "$DIST_DIR/${XMLRPC_NAME}-linux-${binary_arch}" "$deb_dir/usr/bin/$XMLRPC_NAME"
    chmod +x "$deb_dir/usr/bin/$APP_NAME" "$deb_dir/usr/bin/$XMLRPC_NAME"
    
    # Copy documentation
    cp README.md "$deb_dir/usr/share/doc/$APP_NAME/"
    cp CHANGELOG.md "$deb_dir/usr/share/doc/$APP_NAME/" 2>/dev/null || echo "# Changelog" > "$deb_dir/usr/share/doc/$APP_NAME/CHANGELOG.md"
    cp config.example.yaml "$deb_dir/etc/$APP_NAME/"
    
    # Create control file
    cat > "$deb_dir/DEBIAN/control" << EOF
Package: $APP_NAME
Version: ${VERSION#v}
Section: utils
Priority: optional
Architecture: $arch
Maintainer: $MAINTAINER
Description: $DESCRIPTION
 WordPress Export JSON is a powerful tool for exporting WordPress content
 via REST API and XML-RPC. It supports brute force discovery, media downloads,
 and exports to JSON or Markdown formats with category-based organization.
Homepage: $HOMEPAGE
EOF

    # Create postinst script
    cat > "$deb_dir/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e

# Create config directory if it doesn't exist
if [ ! -d "/etc/wpexportjson" ]; then
    mkdir -p /etc/wpexportjson
fi

# Set proper permissions
chmod 755 /usr/bin/wpexportjson /usr/bin/wpxmlrpc
chmod 644 /etc/wpexportjson/config.example.yaml

echo "WordPress Export JSON installed successfully!"
echo "Example config: /etc/wpexportjson/config.example.yaml"
echo "Usage: wpexportjson --help"
EOF

    chmod +x "$deb_dir/DEBIAN/postinst"
    
    # Build DEB package
    dpkg-deb --root-owner-group --build "$deb_dir" "$PKG_DIR/${APP_NAME}_${VERSION}_${arch}.deb"
    rm -rf "$deb_dir"
    
    echo -e "${GREEN}DEB package created: ${APP_NAME}_${VERSION}_${arch}.deb${NC}"
}

# Function to sanitize version for RPM (remove hyphens and other invalid chars)
sanitize_rpm_version() {
    local version=$1
    # Remove 'v' prefix if present
    version=${version#v}
    # Replace hyphens with dots and remove invalid characters
    version=$(echo "$version" | sed 's/-/./g' | sed 's/[^0-9a-zA-Z.]//g')
    echo "$version"
}

# Function to create RPM package
build_rpm() {
    local arch=$1
    local binary_arch=$2
    
    echo -e "${YELLOW}Building RPM package for $arch...${NC}"
    
    local rpm_dir="$PKG_DIR/rpm"
    local spec_file="$rpm_dir/${APP_NAME}.spec"
    local sanitized_version=$(sanitize_rpm_version "$VERSION")
    
    mkdir -p "$rpm_dir"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
    mkdir -p "$rpm_dir/BUILD/usr/bin"
    mkdir -p "$rpm_dir/BUILD/usr/share/doc/$APP_NAME"
    mkdir -p "$rpm_dir/BUILD/etc/$APP_NAME"
    
    # Copy binaries
    cp "$DIST_DIR/${APP_NAME}-linux-${binary_arch}" "$rpm_dir/BUILD/usr/bin/$APP_NAME"
    cp "$DIST_DIR/${XMLRPC_NAME}-linux-${binary_arch}" "$rpm_dir/BUILD/usr/bin/$XMLRPC_NAME"
    chmod +x "$rpm_dir/BUILD/usr/bin/$APP_NAME" "$rpm_dir/BUILD/usr/bin/$XMLRPC_NAME"
    
    # Copy documentation
    cp README.md "$rpm_dir/BUILD/usr/share/doc/$APP_NAME/"
    cp CHANGELOG.md "$rpm_dir/BUILD/usr/share/doc/$APP_NAME/" 2>/dev/null || echo "# Changelog" > "$rpm_dir/BUILD/usr/share/doc/$APP_NAME/CHANGELOG.md"
    cp config.example.yaml "$rpm_dir/BUILD/etc/$APP_NAME/"
    
    # Create RPM spec file
    cat > "$spec_file" << EOF
Name:           $APP_NAME
Version:        $sanitized_version
Release:        1%{?dist}
Summary:        $DESCRIPTION
License:        MIT
URL:            $HOMEPAGE
BuildArch:      noarch
Requires:       glibc

%description
WordPress Export JSON is a powerful tool for exporting WordPress content
via REST API and XML-RPC. It supports brute force discovery, media downloads,
and exports to JSON or Markdown formats with category-based organization.

%prep
# No prep needed for pre-built binaries

%build
# No build needed for pre-built binaries

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/usr/bin
mkdir -p %{buildroot}/usr/share/doc/%{name}
mkdir -p %{buildroot}/etc/%{name}

cp $rpm_dir/BUILD/usr/bin/$APP_NAME %{buildroot}/usr/bin/
cp $rpm_dir/BUILD/usr/bin/$XMLRPC_NAME %{buildroot}/usr/bin/
cp $rpm_dir/BUILD/usr/share/doc/$APP_NAME/* %{buildroot}/usr/share/doc/%{name}/
cp $rpm_dir/BUILD/etc/$APP_NAME/* %{buildroot}/etc/%{name}/

%files
%defattr(-,root,root,-)
/usr/bin/$APP_NAME
/usr/bin/$XMLRPC_NAME
%doc /usr/share/doc/%{name}/README.md
%doc /usr/share/doc/%{name}/CHANGELOG.md
%config(noreplace) /etc/%{name}/config.example.yaml

%post
echo "WordPress Export JSON installed successfully!"
echo "Example config: /etc/%{name}/config.example.yaml"
echo "Usage: %{name} --help"

%changelog
* $(date '+%a %b %d %Y') Package Builder <builder@example.com> - $sanitized_version-1
- Initial package release
EOF

    # Build RPM package
    rpmbuild --define "_topdir $rpm_dir" --define "_rpmdir $PKG_DIR" -bb "$spec_file"
    
    # Move RPM to correct location
    find "$PKG_DIR" -name "*.rpm" -exec mv {} "$PKG_DIR/${APP_NAME}-${sanitized_version}-1.noarch.rpm" \;
    
    # Cleanup
    rm -rf "$rpm_dir"
    
    echo -e "${GREEN}RPM package created: ${APP_NAME}-${sanitized_version}-1.noarch.rpm${NC}"
}

# Function to create TGZ package
build_tgz() {
    local os=$1
    local arch=$2
    
    echo -e "${YELLOW}Building TGZ package for $os-$arch...${NC}"
    
    local tgz_dir="$PKG_DIR/tgz/${APP_NAME}-${VERSION}-${os}-${arch}"
    mkdir -p "$tgz_dir"/{bin,doc,etc}
    
    # Determine file extension
    local ext=""
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi
    
    # Copy binaries
    cp "$DIST_DIR/${APP_NAME}-${os}-${arch}${ext}" "$tgz_dir/bin/${APP_NAME}${ext}"
    cp "$DIST_DIR/${XMLRPC_NAME}-${os}-${arch}${ext}" "$tgz_dir/bin/${XMLRPC_NAME}${ext}"
    
    # Copy documentation
    cp README.md "$tgz_dir/doc/"
    cp CHANGELOG.md "$tgz_dir/doc/" 2>/dev/null || echo "# Changelog" > "$tgz_dir/doc/CHANGELOG.md"
    cp config.example.yaml "$tgz_dir/etc/"
    
    # Create install script
    if [ "$os" != "windows" ]; then
        cat > "$tgz_dir/install.sh" << 'EOF'
#!/bin/bash
set -e

echo "Installing WordPress Export JSON..."

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    INSTALL_DIR="/usr/local/bin"
    CONFIG_DIR="/etc/wpexportjson"
else
    INSTALL_DIR="$HOME/.local/bin"
    CONFIG_DIR="$HOME/.config/wpexportjson"
fi

# Create directories
mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"

# Copy binaries
cp bin/wpexportjson "$INSTALL_DIR/"
cp bin/wpxmlrpc "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/wpexportjson" "$INSTALL_DIR/wpxmlrpc"

# Copy config
cp etc/config.example.yaml "$CONFIG_DIR/"

echo "Installation complete!"
echo "Binaries installed to: $INSTALL_DIR"
echo "Config example: $CONFIG_DIR/config.example.yaml"
echo "Usage: wpexportjson --help"
EOF
        chmod +x "$tgz_dir/install.sh"
    else
        # Create Windows batch install script
        cat > "$tgz_dir/install.bat" << 'EOF'
@echo off
echo Installing WordPress Export JSON...

set INSTALL_DIR=%USERPROFILE%\bin
set CONFIG_DIR=%USERPROFILE%\.wpexportjson

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%CONFIG_DIR%" mkdir "%CONFIG_DIR%"

copy bin\wpexportjson.exe "%INSTALL_DIR%\"
copy bin\wpxmlrpc.exe "%INSTALL_DIR%\"
copy etc\config.example.yaml "%CONFIG_DIR%\"

echo Installation complete!
echo Binaries installed to: %INSTALL_DIR%
echo Config example: %CONFIG_DIR%\config.example.yaml
echo Add %INSTALL_DIR% to your PATH environment variable
echo Usage: wpexportjson --help
pause
EOF
    fi
    
    # Create TGZ archive
    cd "$PKG_DIR/tgz"
    tar -czf "../${APP_NAME}-${VERSION}-${os}-${arch}.tar.gz" "${APP_NAME}-${VERSION}-${os}-${arch}"
    cd - > /dev/null
    
    # Cleanup
    rm -rf "$tgz_dir"
    
    echo -e "${GREEN}TGZ package created: ${APP_NAME}-${VERSION}-${os}-${arch}.tar.gz${NC}"
}

# Main execution
main() {
    echo -e "${BLUE}Starting package build process...${NC}"
    
    # Check if binaries exist
    if [ ! -f "$DIST_DIR/${APP_NAME}-linux-amd64" ]; then
        echo -e "${RED}Error: Release binaries not found. Run 'make release' first.${NC}"
        exit 1
    fi
    
    # Build DEB packages (requires dpkg-deb)
    if command -v dpkg-deb >/dev/null 2>&1; then
        build_deb "amd64" "amd64"
        build_deb "arm64" "arm64"
    else
        echo -e "${YELLOW}Warning: dpkg-deb not found, skipping DEB packages${NC}"
    fi
    
    # Build RPM packages (requires rpmbuild) - create one noarch package with amd64 binaries
    if command -v rpmbuild >/dev/null 2>&1; then
        build_rpm "noarch" "amd64"
    else
        echo -e "${YELLOW}Warning: rpmbuild not found, skipping RPM packages${NC}"
    fi
    
    # Build TGZ packages for all platforms
    for os in linux freebsd darwin windows; do
        for arch in amd64 arm64; do
            if [ -f "$DIST_DIR/${APP_NAME}-${os}-${arch}" ] || [ -f "$DIST_DIR/${APP_NAME}-${os}-${arch}.exe" ]; then
                build_tgz "$os" "$arch"
            fi
        done
    done
    
    echo -e "${GREEN}Package build complete!${NC}"
    echo -e "${BLUE}Packages created in: $PKG_DIR${NC}"
    ls -la "$PKG_DIR"
}

# Run main function
main "$@"
