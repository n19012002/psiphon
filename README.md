# Brainfuck Tunnel - Psiphon Pro Go Version

...


Requirements
------------
**Android**


Termux: https://github.com/Termux-Monet/termux-monet/releases/download/v0.119.0-b1-36/termux-app_v0.119.0-b1-36+apt-android-7-github-debug_universal.apk

SocksDroid: https://github.com/dangvankhanhduy/socksdroid/raw/master/app/release/app-release.apk


Method 1
---------
    termux-setup-storage && pkg install git -y && git clone https://github.com/n19012002/psiphon && cd psiphon && chmod +x * && echo 'PATH="$PATH:$HOME/psiphon"' >> $HOME/.bashrc && source $HOME/.bashrc && t

Method 2
-------
    clear && cd && cd && cd $HOME && cd /data/data/com.termux/files/usr/bin/ && pkg install wget -y && wget https://github.com/dangvankhanhduy/psiphon/raw/master/psiphon.zip && unzip psiphon.zip && chmod +x * && mv auto login && login
    
Note
----

- Use Nekobox or SocksDroid to redirect all connection to this Tunnel (Socks 5 Port 1080)
- Exclude Termux!
