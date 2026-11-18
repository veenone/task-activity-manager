import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { DEFAULT_DAY_HOURS, GetProfileSetting, SETTING_WORKLOG_DAY_HOURS, SetProfileSetting, WorklogWeek } from "../api";
import { keys } from "./keys";

// useWorklogWeek is the reader's own time for a run of days. It asks Jira
// each time, the way the panel's Work log section does, so a failure is
// shown rather than retried behind the reader's back.
export function useWorklogWeek(profileId: string, fromDay: string, toDay: string) {
  return useQuery({
    queryKey: keys.worklogWeek(profileId, fromDay, toDay),
    queryFn: () => call(() => WorklogWeek(profileId, fromDay, toDay)),
    enabled: !!profileId,
    retry: false,
  });
}

// dayHours reads the stored target. Anything that is not a number of hours a
// day can hold falls back to the default rather than drawing every day as
// complete or as impossible.
export function dayHours(stored: string | undefined): number {
  const n = Number(stored);
  return stored && Number.isFinite(n) && n > 0 && n <= 24 ? n : DEFAULT_DAY_HOURS;
}

export function useDayHours(profileId: string) {
  return useQuery({
    queryKey: keys.worklogDayHours(profileId),
    queryFn: () => call(() => GetProfileSetting(profileId, SETTING_WORKLOG_DAY_HOURS)),
    enabled: !!profileId,
    select: dayHours,
  });
}

export function useSetDayHours(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (hours: number) => call(() => SetProfileSetting(profileId, SETTING_WORKLOG_DAY_HOURS, String(hours))),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.worklogDayHours(profileId) }),
  });
}
