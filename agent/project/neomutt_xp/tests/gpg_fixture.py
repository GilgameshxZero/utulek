"""Create genuine encrypted fixture data, exclusively with test credentials."""
from pathlib import Path
import subprocess

def encrypt_fixture(directory, filename, data):
    directory=Path(directory)
    directory.mkdir(parents=True,exist_ok=True)
    home=directory/'gpg_home';home.mkdir(exist_ok=True);home.chmod(0o700)
    password=directory/'passphrase.txt';password.write_text('fixture-only-passphrase\n');password.chmod(0o600)
    target=directory/filename
    command=['gpg','--homedir',str(home),'--batch','--yes','--quiet','--no-symkey-cache',
             '--pinentry-mode','loopback','--passphrase-file',str(password),
             '--compress-algo','none','--symmetric','--output',str(target)]
    subprocess.run(command,input=data,capture_output=True,check=True)
    target.chmod(0o600)
    return target

def decrypt_command(directory):
    directory=str(directory)
    return ('gpg --homedir "'+directory+'/gpg_home" --batch --quiet --no-symkey-cache '
            '--pinentry-mode loopback --passphrase-file "'+directory+'/passphrase.txt" --decrypt')
