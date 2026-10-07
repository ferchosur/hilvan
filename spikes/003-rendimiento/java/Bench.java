// Spike 003 — Java slice of Hilvan (JDK only). The JDK has no JSON parser: jsonl/spawn only
// count lines and extract "type" with a string search, which favours Java (noted in the report).
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.*;

public class Bench {
  static final String[] TEXTS = {
    "Refactor the Codex adapter to normalise JSONL events and add tests",
    "Fix bug in login flow when the token expires",
    "Document the API in the README with examples",
    "Investigate terminal UI alternatives for low resource machines",
    "Add support for SSH destinations in the launcher",
    "Review the authentication pull request for security issues",
    "Increase unit test coverage of the policy evaluator",
    "Implement export of task history to CSV files",
  };
  static final int K = 7, BUCKETS = 65536;

  static long fnv1a(String s) {
    int h = 0x811c9dc5;
    for (int i = 0; i < s.length(); i++) { h ^= s.charAt(i); h *= 0x01000193; }
    return h & 0xffffffffL;
  }
  static List<String> tokenize(String t) {
    List<String> out = new ArrayList<>();
    for (String w : t.toLowerCase().split("[^a-z0-9]+")) {
      if (w.length() < 2) continue;
      out.add(w.length() > 6 ? w.substring(0, 6) : w);
    }
    return out;
  }
  static List<String> features(String t) {
    List<String> toks = tokenize(t), f = new ArrayList<>();
    for (int i = 0; i < toks.size(); i++) {
      f.add("w:" + toks.get(i));
      if (i + 1 < toks.size()) f.add("b:" + toks.get(i) + "_" + toks.get(i + 1));
    }
    return f;
  }
  static double[] weightsFor(long b) {
    double[] w = new double[K];
    for (int c = 0; c < K; c++) w[c] = ((((b * (c + 1) * 2654435761L) & 0xffffffffL) % 1000) / 1000.0) - 0.5;
    return w;
  }
  static void criterio(int n) {
    Map<Long, double[]> W = new HashMap<>();
    for (String t : TEXTS) for (String f : features(t)) { long b = fnv1a(f) % BUCKETS; W.put(b, weightsFor(b)); }
    long check = 0;
    for (int i = 0; i < n; i++) {
      double[] z = new double[K];
      for (String f : features(TEXTS[i % 8] + " " + (i % 100))) {
        double[] w = W.get(fnv1a(f) % BUCKETS);
        if (w != null) for (int c = 0; c < K; c++) z[c] += w[c];
      }
      double m = Double.NEGATIVE_INFINITY; for (double v : z) m = Math.max(m, v);
      int best = 0; double[] p = new double[K];
      for (int c = 0; c < K; c++) { p[c] = Math.exp(z[c] - m); if (p[c] > p[best]) best = c; }
      check += best;
    }
    System.out.println("criterio n=" + n + " check=" + check);
  }
  static void readLines(InputStream in) throws IOException {
    long lines = 0, results = 0;
    BufferedReader r = new BufferedReader(new InputStreamReader(in, StandardCharsets.UTF_8), 1 << 16);
    for (String line; (line = r.readLine()) != null; ) {
      if (line.isEmpty()) continue;
      lines++;
      if (line.startsWith("{\"type\":\"result\"")) results++;
    }
    System.out.println("lines=" + lines + " results=" + results + " (no JSON parsing)");
  }
  public static void main(String[] a) throws Exception {
    switch (a[0]) {
      case "version": System.out.println("hilvan-java 0.0.0"); break;
      case "criterio": criterio(Integer.parseInt(a[1])); break;
      case "jsonl": try (InputStream in = new FileInputStream(a[1])) { readLines(in); } break;
      case "spawn": {
        Process p = new ProcessBuilder("cat", a[1]).redirectError(ProcessBuilder.Redirect.INHERIT).start();
        readLines(p.getInputStream()); p.waitFor(); break;
      }
      default: System.exit(2);
    }
  }
}
