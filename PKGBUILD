pkgname=desktopimage
pkgver=1.0.0
pkgrel=1
pkgdesc="A tool to automatically generate .desktop files for AppImage applications."
arch=('x86_64')
url="https://github.com/lrx0014/DesktopImage"
license=('MIT')
depends=('glibc')
makedepends=('git' 'go')
source=("git+$url.git#branch=master")
sha256sums=('SKIP')

pkgver() {
    cd "$srcdir/$pkgname"
    git describe --tags --always | sed 's/^v//'
}

build() {
    cd "$srcdir/$pkgname/src"
    go build -o desktopimage main.go
}

package() {
    install -Dm755 "$srcdir/$pkgname/src/desktopimage" "$pkgdir/usr/bin/desktopimage"
    install -Dm644 "$srcdir/$pkgname/desktopimage.service" "$pkgdir/etc/systemd/system/desktopimage.service"
}