# All the wait functions share the same timeouts
readonly wait_for_retries=60
readonly wait_for_sleep=2

wait_for_port() {
    local port=$1
    local hostname=$2
    # Use localhost by default
    : ${hostname:=localhost}

    for retry in $(seq $wait_for_retries); do
        if nc -z $hostname $port > /dev/null 2>&1; then
            return
        else
            sleep $wait_for_sleep
        fi
    done

    echo "Port $port is not available"
    # debug - check logs because connection to 6443 is refused for some reason
    echo "\n etcd.log \n"
    cat etcd.log
    echo "\n apiserver.log \n"
    cat apiserver.log
    echo "\n controller-manager.log \n"
    cat controller-manager.log
    echo "\n scheduler.log \n"
    cat scheduler.log
    exit 2
}

# Usage: wait_for_file filename1 filename2 ...
wait_for_file() {
    for filename in $@; do
        for retry in $(seq $wait_for_retries); do
            if [ -e $filename ]; then
                return
            else
                sleep $wait_for_sleep
            fi
        done
    done

    log "ERROR: File $filename is not available"
    exit 2
}

# Tries to get a non-empty output from the passed in command
get_cmd_output () {
    local output=""
    local cmd=$@
    for i in $(seq 1 $wait_for_retries); do
    	output=$($cmd) || continue
    	if [[ ! -z "$output" ]]; then
    	    echo -n $output
    	    return 0
    	fi
       	sleep $wait_for_sleep
    done
    return 1
}

readonly _new_line_char="\n"

# Write message to stderr, adding a \n if required
log() {
    local line="$*"
    printf '%s\n' "${line%$_new_line_char}" 1>&2
}

# Print error message on stderr and exit 1
die() {
    printf "%s\n" "$*" > /dev/stderr
    exit 1
}

# Usage: http_download <url> <filename>
# If <filename> already exists, it assumes it has been downloaded
http_download() {
    local url="$1"
    local output="$2"
    if [ -z "$output" ]
    then
        output=$(basename $url)
    fi

    if ! [ -r "$output" ]
    then
        curl -OL --progress-bar "$url" -o "$output"
    fi
    echo "downloaded $url to $output"
}
