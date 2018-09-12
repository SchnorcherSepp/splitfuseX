#!/bin/bash
VERSION="3.14"
 
DLURL="https://github.com/SchnorcherSepp/splitfuseX/releases/download/$VERSION/splitfuseX"
INSTALLPATH="/usr/bin/splitfuse"
SERVICEFILE="/etc/systemd/system/splitfuse.service"
CONFIGDIR="/etc/splitfuse"
CONFIGFILE="$CONFIGDIR/splitfuse.conf"
MOUNTDIR="/mnt/splitfuse"
USER=splitfuse
 
 
##################################################
#  check user root                               #
##################################################
if [ "$(/usr/bin/whoami)" != "root" ]; then
   /bin/echo "This script must be run as root" 1>&2
   exit 1
fi
 
##################################################
#  add new system user                           #
##################################################
/bin/echo "add new system user: $USER"
/usr/sbin/adduser --system --home /etc/empty --no-create-home --disabled-login --disabled-password --group $USER > /dev/null
 
##################################################
#  dependencies                                  #
##################################################
/bin/echo "install and config FUSE"
/usr/bin/apt update          &> /dev/null
/usr/bin/apt install fuse -y &> /dev/null
/bin/sed -i "s/#user_allow_other/user_allow_other/g" /etc/fuse.conf
 
##################################################
#  install splitfuse                             #
##################################################
/bin/echo "download and install splitfuseX"
/usr/bin/wget -q -O $INSTALLPATH $DLURL
/bin/chmod +x $INSTALLPATH
 
##################################################
#  tests and exit                                #
##################################################
/bin/echo "VERSION:"
$INSTALLPATH --version
if [ $? -ne 0 ]; then
  exit 1
fi
 
 
##################################################
#  service & config                              #
##################################################
/bin/echo "write service and config files"
 
/bin/mkdir -p $CONFIGDIR
/bin/chown $USER:$USER $CONFIGDIR
/bin/mkdir -p $MOUNTDIR
/bin/chown $USER:$USER $MOUNTDIR
 
 
if [ ! -f $CONFIGFILE ]; then
cat > $CONFIGFILE << EOL
# splitfuse config used by $SERVICEFILE
# see $INSTALLPATH --help for more details
 
SPLITFUSE_dir=/mnt/splitfuse
SPLITFUSE_key=$CONFIGDIR/splitfuse.key
SPLITFUSE_cache=$CONFIGDIR/cache.dat
SPLITFUSE_token=$CONFIGDIR/token.json
SPLITFUSE_client=$CONFIGDIR/client_secret.json
SPLITFUSE_dbname=index.db
SPLITFUSE_module=drive
SPLITFUSE_chunks=root
EOL
fi
 
 
cat > $SERVICEFILE << EOL
# SpliFuseX mount on boot (edit config: $CONFIGFILE)
# Register new service by typing:
#    systemctl daemon-reload
#    systemctl enable splitfuse.service
#    systemctl start splitfuse.service
# To unmount use:
#    systemctl stop splitfuse.service
# To mount use:
#    systemctl start splitfuse.service
# show status
#    systemctl status splitfuse.service
 
[Unit]
Description=SplitFuseX mount
Documentation=https://github.com/SchnorcherSepp/splitfuseX
After=multi-user.target
 
[Service]
EnvironmentFile=-$CONFIGFILE
Type=simple
User=$USER
ExecStart=$INSTALLPATH mount \
  --module \$SPLITFUSE_module \
  --dir \$SPLITFUSE_dir \
  --chunks \$SPLITFUSE_chunks \
  --dbfileName \$SPLITFUSE_dbname \
  --client \$SPLITFUSE_client \
  --token \$SPLITFUSE_token \
  --key \$SPLITFUSE_key \
  --cache \$SPLITFUSE_cache
ExecStop=/bin/fusermount -uz \$SPLITFUSE_dir
 
[Install]
WantedBy=multi-user.target
EOL
