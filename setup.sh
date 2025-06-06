# update
apt update && apt upgrade -y

# install posrgresql
apt install postgresql

# log as postgres automatically created user
sudo -i -u postgres
# PostgreSQL shell (psql)
psql
# run db_init.sql commands
# quit psql shell
\q

# install go lang
apt install golang-go

# copy files from local if needed
scp your_file.py yourname@your.server.ip.address:/home/yourname/


ZeleNina2025!