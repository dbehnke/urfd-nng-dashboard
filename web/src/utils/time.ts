export function formatTimeSince(date: string | Date | undefined): string {
    if (!date) return '-';

    const now = new Date();
    const then = new Date(date);
    const diffMs = now.getTime() - then.getTime();

    if (diffMs < 0) return 'Just now';

    const seconds = Math.floor(diffMs / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) {
        return `${days}d ${hours % 24}h`;
    }
    if (hours > 0) {
        return `${hours}h ${minutes % 60}m`;
    }
    if (minutes > 0) {
        return `${minutes}m ${seconds % 60}s`;
    }
    return `${seconds}s`;
}
