package mount

import "syscall"

const (
	F_OFD_GETLK  = 36
	F_OFD_SETLK  = 37
	F_OFD_SETLKW = 38

	S_IRUGO   = syscall.S_IRGRP | syscall.S_IRUSR | syscall.S_IROTH
	S_IWUGO   = syscall.S_IWGRP | syscall.S_IWUSR | syscall.S_IWOTH
	S_IXUGO   = syscall.S_IXGRP | syscall.S_IXUSR | syscall.S_IXOTH
	S_IRWXUGO = syscall.S_IRWXU | syscall.S_IRWXG | syscall.S_IRWXO
	F_DIR_RW  = syscall.S_IFDIR | 0777
	F_FILE_RW = syscall.S_IFREG | 0644
)
