# update
apt update && apt upgrade -y

# install posrgresql
apt install postgresql

# log as postgres automatically created user
sudo -i -u postgres
# PostgreSQL shell (psql)
psql
# psql -U trd_user -d trd_db -h localhost
# run db_init.sql commands
# quit psql shell
\q

# install go lang
apt install golang-go
# install go lang 1.24 on arm64
wget https://go.dev/dl/go1.24.0.linux-arm64.tar.gz
tar -C /usr/local -xzf go1.24.0.linux-arm64.tar.gz
export PATH=$PATH:/usr/local/go/bin (add to ~/.bashrc, ~/.profile )

# copy files from local if needed
scp your_file.py yourname@your.server.ip.address:/home/yourname/



# Install Nginx
sudo apt update
sudo apt install nginx
# Check it works
systemctl status nginx
# Visit your VPS IP in browser -> You should see the default Nginx welcome page
# Create a new config file for your Go app
sudo nano /etc/nginx/sites-available/trd-01-backend
# Create a symlink in sites-enabled:
sudo ln -s /etc/nginx/sites-available/myapp /etc/nginx/sites-enabled/
# Check for config errors
sudo nginx -t
# If all good, reload
sudo systemctl reload nginx