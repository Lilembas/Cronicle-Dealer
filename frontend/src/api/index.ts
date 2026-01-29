import request from './request'

export interface Job {
    id: string
    name: string
    description: string
    category: string
    cron_expr: string
    timezone: string
    enabled: boolean
    task_type: string
    command: string
    working_dir: string
    env: string
    target_type: string
    target_value: string
    timeout: number
    max_retries: number
    retry_delay: number
    concurrent: boolean
    queue_max_size: number
    notify_on_success: boolean
    notify_on_failure: boolean
    notify_webhook: string
    created_by: string
    updated_by: string
    created_at: string
    updated_at: string
    last_run_time?: string
    next_run_time?: string
    total_runs: number
    success_runs: number
    failed_runs: number
}

export interface JobListResponse {
    total: number
    page: number
    data: Job[]
}

export interface Event {
    id: string
    job_id: string
    job_name: string
    node_id: string
    node_name: string
    status: string
    scheduled_time: string
    start_time?: string
    end_time?: string
    duration: number
    exit_code: number
    error_message: string
    log_path: string
    log_size: number
    cpu_percent: number
    memory_bytes: number
    retry_count: number
    is_retry: boolean
    parent_event_id: string
    created_at: string
    updated_at: string
}

export interface EventListResponse {
    total: number
    page: number
    data: Event[]
}

export interface Node {
    id: string
    hostname: string
    ip: string
    tags: string
    status: string
    cpu_cores: number
    cpu_usage: number
    memory_total: number
    memory_usage: number
    memory_percent: number
    disk_total: number
    disk_usage: number
    disk_percent: number
    running_jobs: number
    max_concurrent: number
    version: string
    last_heartbeat: string
    registered_at: string
    updated_at: string
}

export interface Stats {
    total_jobs: number
    enabled_jobs: number
    total_events: number
    running_events: number
    success_events: number
    failed_events: number
    online_nodes: number
    offline_nodes: number
}

// 任务 API
export const jobsApi = {
    list: (params?: { page?: number; page_size?: number; category?: string; enabled?: boolean }) =>
        request.get<JobListResponse>('/jobs', { params }),

    get: (id: string) =>
        request.get<Job>(`/jobs/${id}`),

    create: (data: Partial<Job>) =>
        request.post<Job>('/jobs', data),

    update: (id: string, data: Partial<Job>) =>
        request.put<Job>(`/jobs/${id}`, data),

    delete: (id: string) =>
        request.delete(`/jobs/${id}`),

    trigger: (id: string) =>
        request.post(`/jobs/${id}/trigger`),
}

// 执行记录 API
export const eventsApi = {
    list: (params?: { page?: number; page_size?: number; job_id?: string; status?: string }) =>
        request.get<EventListResponse>('/events', { params }),

    get: (id: string) =>
        request.get<Event>(`/events/${id}`),

    abort: (id: string) =>
        request.post(`/events/${id}/abort`),
}

// 节点 API
export const nodesApi = {
    list: (params?: { status?: string }) =>
        request.get<Node[]>('/nodes', { params }),

    get: (id: string) =>
        request.get<Node>(`/nodes/${id}`),

    delete: (id: string) =>
        request.delete(`/nodes/${id}`),
}

// 统计 API
export const statsApi = {
    get: () =>
        request.get<Stats>('/stats'),
}

// Shell 执行 API
export interface ShellExecuteRequest {
    command: string
    node_id?: string
    timeout?: number
}

export interface ShellExecuteResponse {
    event_id: string
    job_id: string
    command: string
    status: string
    message: string
    node_id?: string
}

export interface ShellLogsResponse {
    event_id: string
    logs: string
    complete: boolean
    exit_code: number
    status: string
}

export const shellApi = {
    execute: (data: ShellExecuteRequest) =>
        request.post<ShellExecuteResponse>('/shell/execute', data),

    getLogs: (eventId: string) =>
        request.get<ShellLogsResponse>(`/shell/logs/${eventId}`),
}

