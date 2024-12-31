# DesktopImage
Automatically convert **.AppImage** to **.desktop** Application in Linux
> [!WARNING]  
> This tool has only been tested briefly on Arch Linux

## Installation
**Clone this repo:**
```shell
git clone https://github.com/lrx0014/DesktopImage.git
cd DesktopImage
```

**For ArchLinux Users:**
```shell
makepkg -si

sudo systemctl start desktopimage
# if you prefer auto-start at boot
sudo systemctl enable desktopimage
```

**For other distro, you could compile it with go:**
```shell
go build -o DesktopImage src/main.go
```

## Configuration
```shell
# modify the configuration to specify your path to monitor
# otherwise the process will do nothing due to the initiate configuration is empty
vim /etc/desktopimage/config.toml
``` 

## Example
assume that we have a configuration as follows:
```toml
app_path = "/home/me/Downloads"
desktop_path = "/home/me/.local/share/Applications"
icon_path = "/path/to/icon.png" # optional
categories = "Application"
```
When you download a **Test.AppImage** to the **Downloads** directory, a **Test.desktop** file will be automatically generated into the path **/home/me/.local/share/Applications/** and bound to the AppImage, so that you can easily open this program directly in your application launcher. Whenever you remove the AppImage from Downloads, the corresponding **.desktop** file will also be automatically deleted.
