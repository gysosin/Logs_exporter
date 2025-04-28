// -----------------------------------------------------------------------------
//  FolderMonitorEnhanced.cs   (v3 – buffered flush)
//  Target Framework: .NET 6+ | Author: ChatGPT refactor for single‑line output
// -----------------------------------------------------------------------------
//  Goal: guarantee *exactly one* console/log line for a burst of ETW events
//  concerning the same file path.  Approach now uses a small in‑memory buffer
//  and a periodic flush timer rather than emitting immediately on receipt.  A
//  burst is considered settled when no new events for that path arrive within
//  <window> (default 2 s).  Highest‑severity event wins inside the burst.
// -----------------------------------------------------------------------------

using Microsoft.Diagnostics.Tracing;
using Microsoft.Diagnostics.Tracing.Parsers;
using Microsoft.Diagnostics.Tracing.Parsers.Kernel;
using Microsoft.Diagnostics.Tracing.Session;
using Microsoft.Win32.SafeHandles;
using System.Collections.Concurrent;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Text;

namespace FolderMonitor;

internal sealed class Program : IDisposable
{
    // ───────────────────────────── configuration ─────────────────────────────
    private static readonly string DefaultMonDir = @"C:\Users\xyfo\Desktop\test";
    private static readonly string DefaultLogName = "audit.log";
    private static readonly TimeSpan DefaultWindow = TimeSpan.FromSeconds(2);
    private static readonly TimeSpan FlushInterval = TimeSpan.FromMilliseconds(500); // timer tick
    private static readonly string[] DefaultNoise = { "desktop.ini", "folder.jpg", "folder.gif" };

    // ───────────────────────────── runtime state ─────────────────────────────
    private const string EtwSessionName = "FileIoAuditSession";
    private readonly string _monDirPrefix;
    private readonly string _logPath;
    private readonly TextWriter _log;
    private readonly FileSystemWatcher _fsw;
    private readonly TraceEventSession _etwSession;
    private readonly Task _etwPump;
    private readonly Timer _flushTimer;

    private readonly ConcurrentDictionary<int, ProcessInfo> _procCache = new();
    private readonly ConcurrentDictionary<string, AggRec> _buffer;  // main aggregator
    private readonly CancellationTokenSource _cts = new();

    private readonly TimeSpan _window;
    private readonly string[] _noiseGlobs;

    // Event severity ranking – higher beats lower
    private static readonly Dictionary<char, int> _rank = new()
    {
        ['D'] = 4,
        ['R'] = 3,
        ['W'] = 2,
        ['C'] = 1
    };

    private sealed class ProcessInfo { public string Name = "unknown"; public string User = "unknown"; }

    private sealed record AggRec(DateTime Ts, char Evt, int Pid);

    // ───────────────────────────────── public API ────────────────────────────
    public static int Main(string[] args)
    {
        var dir = args.ElementAtOrDefault(0) ?? DefaultMonDir;
        var log = args.ElementAtOrDefault(1) ?? DefaultLogName;
        var winMs = double.TryParse(args.ElementAtOrDefault(2), out var ms) ? ms : DefaultWindow.TotalMilliseconds;

        if (!Directory.Exists(dir)) { Console.Error.WriteLine($"Dir '{dir}' not found."); return 2; }
        if (!(TraceEventSession.IsElevated() ?? false)) { Console.Error.WriteLine("Run as admin."); return 1; }

        using var m = new Program(dir, log, TimeSpan.FromMilliseconds(winMs), DefaultNoise);
        Console.WriteLine($"Auditing '{dir}' … (service mode detected: {Environment.UserInteractive == false})");

            if (Environment.UserInteractive)
            {
                Console.WriteLine("Press <Enter> to stop.");
                Console.ReadLine();
            }
            else
            {
                // Running as service: block forever until process is killed
                new ManualResetEvent(false).WaitOne();
            }

            return 0;

    }

    private Program(string monDir, string logFile, TimeSpan window, string[] noise)
    {
        _window = window;
        _noiseGlobs = noise;
        _buffer = new(StringComparer.OrdinalIgnoreCase);

        _monDirPrefix = Path.GetFullPath(monDir.TrimEnd('\\')) + '\\';
        _logPath = Path.Combine(monDir, logFile);
        _log = TextWriter.Synchronized(new StreamWriter(_logPath, true) { AutoFlush = true });
        _log.WriteLine($"=== Session started {DateTime.Now:yyyy-MM-dd HH:mm:ss} ===");

        _fsw = CreateQuickWatcher(monDir);
        _etwSession = new TraceEventSession(EtwSessionName) { StopOnDispose = true };
        EnableKernelProviders(_etwSession); HookEtwCallbacks(_etwSession);
        _etwPump = Task.Run(() => _etwSession.Source.Process(), _cts.Token);

        _flushTimer = new Timer(_ => Flush(), null, FlushInterval, FlushInterval);
    }

    public void Dispose()
    {
        _cts.Cancel();
        _flushTimer.Dispose();
        _etwSession.Dispose();
        _fsw.Dispose();
        _log.WriteLine($"=== Session ended   {DateTime.Now:yyyy-MM-dd HH:mm:ss} ===");
        _log.Dispose();
    }

    // ───────────────────────── flush logic (timer) ──────────────────────────
    private void Flush()
    {
        var cutoff = DateTime.UtcNow - _window;
        foreach (var kv in _buffer.ToArray())
        {
            if (kv.Value.Ts < cutoff && _buffer.TryRemove(kv.Key, out var rec))
            {
                if (!_procCache.TryGetValue(rec.Pid, out var p))
                    p = _procCache[rec.Pid] = new ProcessInfo { Name = TryGetName(rec.Pid), User = TryGetOwner(rec.Pid) };

                string msg = $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] [ETW] {EvtName(rec.Evt)} by " +
                             $"{p.Name} (PID {rec.Pid}, User {p.User}): {kv.Key}";
                Console.WriteLine(msg);
                _log.WriteLine(msg);
            }
        }
    }

    // ────────────────────── quick FileSystemWatcher (fallback) ──────────────
    private static FileSystemWatcher CreateQuickWatcher(string dir)
    {
        var w = new FileSystemWatcher(dir)
        {
            Filter = "*.*",
            IncludeSubdirectories = true,
            EnableRaisingEvents = true,
            NotifyFilter = NotifyFilters.FileName | NotifyFilters.Size | NotifyFilters.LastWrite
        };
        w.Created += (_, e) => Console.WriteLine($"[FSW] {e.ChangeType}: {e.FullPath}");
        w.Deleted += (_, e) => Console.WriteLine($"[FSW] {e.ChangeType}: {e.FullPath}");
        w.Changed += (_, e) => Console.WriteLine($"[FSW] {e.ChangeType}: {e.FullPath}");
        w.Renamed += (_, e) => Console.WriteLine($"[FSW] RENAMED: {e.OldFullPath} → {e.FullPath}");
        return w;
    }

    // ───────────────────────────── ETW plumbing ─────────────────────────────
    private static void EnableKernelProviders(TraceEventSession s) =>
        s.EnableKernelProvider(
            KernelTraceEventParser.Keywords.FileIO |
            KernelTraceEventParser.Keywords.FileIOInit |
            KernelTraceEventParser.Keywords.Process);

    private void HookEtwCallbacks(TraceEventSession s)
    {
        var k = s.Source.Kernel;
        k.ProcessStart += d => _procCache[d.ProcessID] = new ProcessInfo { Name = d.ProcessName, User = TryGetOwner(d.ProcessID) };
        k.FileIOCreate += d => Buffer('C', d.ProcessID, d.FileName);
        k.FileIODelete += d => Buffer('D', d.ProcessID, d.FileName);
        k.FileIOWrite += d => Buffer('W', d.ProcessID, d.FileName);
        k.FileIORename += d => Buffer('R', d.ProcessID, d.FileName);
    }

    // ───────────────────── event → buffer (no I/O) ──────────────────────────
    private void Buffer(char evt, int pid, string? raw)
    {
        if (string.IsNullOrWhiteSpace(raw)) return;
        string path; try { path = Path.GetFullPath(raw.Replace('/', '\\')); } catch { return; }
        if (!path.StartsWith(_monDirPrefix, StringComparison.OrdinalIgnoreCase)) return;
        if (path.Length == _monDirPrefix.Length - 1) return;
        string name = Path.GetFileName(path);
        if (path.Equals(_logPath, StringComparison.OrdinalIgnoreCase)) return;
        if (_noiseGlobs.Any(n => name.Equals(n, StringComparison.OrdinalIgnoreCase))) return;
        if (Path.GetExtension(path).Length == 0) return;

        var now = DateTime.UtcNow;
        _buffer.AddOrUpdate(path,
            _ => new AggRec(now, evt, pid),
            (_, old) =>
            {
                // newer within window? overwrite else reset
                if (now - old.Ts > _window) return new AggRec(now, evt, pid);
                return _rank[evt] > _rank[old.Evt] ? new AggRec(now, evt, pid) : old;
            });
    }

    private static string EvtName(char c) => c switch { 'C' => "CREATED", 'W' => "WRITTEN", 'R' => "RENAMED", 'D' => "DELETED", _ => "?" };

    // ─────────────────── helpers: process name & owner (Win32) ───────────────
    [SupportedOSPlatform("windows")]
    private static string TryGetName(int pid) { try { return Process.GetProcessById(pid).ProcessName; } catch { return "unknown"; } }

    [SupportedOSPlatform("windows")]
    private static string TryGetOwner(int pid)
    {
        using var hProc = OpenProcess(PROC_QUERY_LIMITED, false, pid);
        if (hProc.IsInvalid) return "unknown";
        if (!OpenProcessToken(hProc.DangerousGetHandle(), TOKEN_QUERY, out var hTok) || hTok.IsInvalid) return "unknown";
        using (hTok)
        {
            GetTokenInformation(hTok, TOKEN_INFORMATION_CLASS.TokenUser, IntPtr.Zero, 0, out uint len);
            if (len == 0) return "unknown";
            IntPtr buf = Marshal.AllocHGlobal((int)len);
            try
            {
                if (!GetTokenInformation(hTok, TOKEN_INFORMATION_CLASS.TokenUser, buf, len, out _)) return "unknown";
                var tu = Marshal.PtrToStructure<TOKEN_USER>(buf);
                uint n = 0, d = 0; LookupAccountSid(null, tu.User.Sid, null!, ref n, null!, ref d, out _);
                var name = new StringBuilder((int)n); var dom = new StringBuilder((int)d);
                return LookupAccountSid(null, tu.User.Sid, name, ref n, dom, ref d, out _) ? $"{dom}\\{name}" : "unknown";
            }
            finally { Marshal.FreeHGlobal(buf); }
        }
    }

    // ─────────────────────────── Win32 P/Invoke ─────────────────────────────
    private const uint PROC_QUERY_LIMITED = 0x1000, TOKEN_QUERY = 0x0008;
    [DllImport("kernel32.dll", SetLastError = true)] private static extern SafeProcessHandle OpenProcess(uint acc, bool inh, int pid);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool OpenProcessToken(IntPtr p, uint acc, out SafeTokenHandle tok);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool GetTokenInformation(SafeTokenHandle tok, TOKEN_INFORMATION_CLASS cls, IntPtr buf, uint len, out uint ret);
    [DllImport("advapi32.dll", CharSet = CharSet.Auto, SetLastError = true)] private static extern bool LookupAccountSid(string? sys, IntPtr sid, StringBuilder name, ref uint nLen, StringBuilder dom, ref uint dLen, out SID_NAME_USE use);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern bool CloseHandle(IntPtr h);
    private sealed class SafeTokenHandle : SafeHandleZeroOrMinusOneIsInvalid { private SafeTokenHandle() : base(true) { } protected override bool ReleaseHandle() => CloseHandle(handle); }
    private enum TOKEN_INFORMATION_CLASS { TokenUser = 1 }
    private enum SID_NAME_USE { SidTypeUser = 1 }
    [StructLayout(LayoutKind.Sequential)] private struct SID_AND_ATTRIBUTES { public IntPtr Sid; public uint Attributes; }
    [StructLayout(LayoutKind.Sequential)] private struct TOKEN_USER { public SID_AND_ATTRIBUTES User; }
}
