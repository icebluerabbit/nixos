# shellcheck shell=bash
# Export /proc/fs/fscache/stats as node-exporter textfile-collector metrics.
#
# Args: [--stats PATH] [--out-dir DIR]
# Writes <out-dir>/fscache.prom atomically (tmp + rename) so node-exporter
# never scrapes a partial file. No-op (exit 0) when the stats file is absent,
# which is the case whenever the cachefiles module isn't loaded.

stats=/proc/fs/fscache/stats
dir=/var/lib/node-exporter-textfile

while [ "$#" -gt 0 ]; do
  case "$1" in
  --stats)
    stats="$2"
    shift 2
    ;;
  --out-dir)
    dir="$2"
    shift 2
    ;;
  *)
    echo "unknown argument: $1" >&2
    exit 2
    ;;
  esac
done

[ -r "$stats" ] || exit 0

mkdir -p "$dir"
tmp=$(mktemp "$dir/.fscache.prom.XXXXXX")

awk '
  /^Cookies/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="n")c=a[2]}}
  /^DownOps/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="DL")dl=a[2]}}
  /^CaRdOps/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="RD")rd=a[2]}}
  /^CaWrOps/{for(i=1;i<=NF;i++){split($i,a,"=");if(a[1]=="WR")wr=a[2]}}
  END{
    print "# HELP fscache_read_hits_total Reads served from local fscache (cache hits)."
    print "# TYPE fscache_read_hits_total counter"
    printf "fscache_read_hits_total %d\n", rd+0
    print "# HELP fscache_downloads_total Reads fetched from the NFS server (cache misses)."
    print "# TYPE fscache_downloads_total counter"
    printf "fscache_downloads_total %d\n", dl+0
    print "# HELP fscache_writes_total Objects written into local fscache."
    print "# TYPE fscache_writes_total counter"
    printf "fscache_writes_total %d\n", wr+0
    print "# HELP fscache_cookies Active fscache cookies (objects tracked)."
    print "# TYPE fscache_cookies gauge"
    printf "fscache_cookies %d\n", c+0
  }
' "$stats" >"$tmp"

chmod 0644 "$tmp"
mv -f "$tmp" "$dir/fscache.prom"
