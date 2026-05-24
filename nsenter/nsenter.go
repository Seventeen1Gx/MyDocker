package nsenter

/*
#define _GNU_SOURCE
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>

__attribute__((constructor)) void enter_namespace(void) {
    char *mydocker_pid = getenv("mydocker_pid");
    if (!mydocker_pid) {
        return;
    }

    char *mydocker_cmd = getenv("mydocker_cmd");
    if (!mydocker_cmd) {
        return;
    }

    printf("nsenter: entering namespaces for PID=%s\n", mydocker_pid);

    int i;
    char nspath[1024];
    char *namespaces[] = {"ipc", "uts", "net", "pid", "mnt"};

    for (i = 0; i < 5; i++) {
        sprintf(nspath, "/proc/%s/ns/%s", mydocker_pid, namespaces[i]);
        printf("nsenter: opening namespace: %s\n", nspath);

        int fd = open(nspath, O_RDONLY);
        if (fd == -1) {
            fprintf(stderr, "nsenter: failed to open %s: %s\n", nspath, strerror(errno));
            continue;  // 跳过失败的命名空间，继续尝试其他的
        }

        printf("nsenter: setns for %s (fd=%d)\n", namespaces[i], fd);
        if (setns(fd, 0) == -1) {
            fprintf(stderr, "nsenter: setns failed for %s: %s\n", namespaces[i], strerror(errno));
            close(fd);
            exit(1);
        }
        close(fd);
        printf("nsenter: successfully entered %s namespace\n", namespaces[i]);
    }

    printf("nsenter: executing command: %s\n", mydocker_cmd);
    int ret = system(mydocker_cmd);
    if (ret == -1) {
        fprintf(stderr, "nsenter: system() failed: %s\n", strerror(errno));
    } else if (ret != 0) {
        fprintf(stderr, "nsenter: command exited with status %d\n", ret);
    }

    printf("nsenter: exiting\n");
    exit(0);
}
*/
import "C"
