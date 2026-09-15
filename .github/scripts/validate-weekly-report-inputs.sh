#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 [report_week_start] <dry_run>" >&2
  exit 2
}

if [[ $# -ne 2 ]]; then
  usage
fi

report_week_start=$1
dry_run=$2

if [[ "$dry_run" != "true" && "$dry_run" != "false" ]]; then
  echo "ERROR: dry_run must be exactly true or false" >&2
  exit 1
fi

if [[ -z "$report_week_start" ]]; then
  exit 0
fi

if [[ ! "$report_week_start" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
  echo "ERROR: report_week_start must use YYYY-MM-DD" >&2
  exit 1
fi

year=$((10#${report_week_start:0:4}))
month=$((10#${report_week_start:5:2}))
day=$((10#${report_week_start:8:2}))

if (( month < 1 || month > 12 )); then
  echo "ERROR: report_week_start has an invalid month" >&2
  exit 1
fi

case "$month" in
  2)
    days_in_month=28
    if (( year % 400 == 0 || (year % 4 == 0 && year % 100 != 0) )); then
      days_in_month=29
    fi
    ;;
  4|6|9|11) days_in_month=30 ;;
  *) days_in_month=31 ;;
esac

if (( day < 1 || day > days_in_month )); then
  echo "ERROR: report_week_start has an invalid day" >&2
  exit 1
fi

# Sakamoto's algorithm returns 0 for Sunday and 1 for Monday.
month_offsets=(0 3 2 5 0 3 5 1 4 6 2 4)
dow_year=$year
if (( month < 3 )); then
  dow_year=$((dow_year - 1))
fi
day_of_week=$(( (dow_year + dow_year / 4 - dow_year / 100 + dow_year / 400 + month_offsets[month - 1] + day) % 7 ))

if (( day_of_week != 1 )); then
  echo "ERROR: report_week_start must be a Monday" >&2
  exit 1
fi
