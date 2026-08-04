.pragma library

// Lightweight fenced-code splitter + token highlighter for chat bubbles.
// Returns RichText HTML using caller-supplied theme colors — no deps.

function splitFences(md) {
    if (md === undefined || md === null || md === "")
        return [{ type: "md", text: "" }]
    var text = String(md)
    var parts = []
    var re = /```([a-zA-Z0-9_+-]*)[ \t]*\r?\n([\s\S]*?)```/g
    var last = 0
    var m
    while ((m = re.exec(text)) !== null) {
        if (m.index > last)
            parts.push({ type: "md", text: text.substring(last, m.index) })
        parts.push({ type: "code", lang: (m[1] || "").toLowerCase(), text: m[2] })
        last = m.index + m[0].length
    }
    if (last < text.length)
        parts.push({ type: "md", text: text.substring(last) })
    if (parts.length === 0)
        parts.push({ type: "md", text: text })
    return parts
}

function escapeHtml(s) {
    return String(s)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
}

function keywordsFor(lang) {
    switch (lang) {
    case "go":
        return "break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var true false nil iota"
    case "py": case "python":
        return "False None True and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield"
    case "js": case "javascript": case "ts": case "typescript":
        return "async await break case catch class const continue debugger default delete do else export extends finally for function if import in instanceof let new return static super switch this throw try typeof var void while with yield true false null undefined"
    case "bash": case "sh": case "shell": case "zsh":
        return "if then else elif fi for while in do done case esac function return export local readonly true false"
    case "c": case "cpp": case "c++": case "cxx": case "h": case "hpp":
        return "alignas alignof and and_eq asm auto bitand bitor bool break case catch char class compl const consteval constexpr continue default delete do double dynamic_cast else enum explicit export extern false float for friend goto if inline int long mutable namespace new noexcept not not_eq nullptr operator or or_eq private protected public register reinterpret_cast return short signed sizeof static static_assert static_cast struct switch template this throw true try typedef typeid typename union unsigned using virtual void volatile wchar_t while xor xor_eq"
    case "rs": case "rust":
        return "as async await break const continue crate dyn else enum extern false fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait true type unsafe use where while yield"
    case "sql":
        return "add all alter and any as asc between by case check column create default delete desc distinct drop else end exists false foreign from full group having in index inner insert into is join key left like limit not null on or order outer primary references right select set table then true union unique update values when where"
    case "json":
        return "true false null"
    case "qml":
        return "as break case catch continue debugger default delete do else finally for function if import in instanceof let new return switch this throw try typeof var void while with true false null undefined readonly property signal alias enum"
    default:
        return ""
    }
}

function commentStyle(lang) {
    switch (lang) {
    case "py": case "python": case "bash": case "sh": case "shell": case "zsh":
        return "hash"
    case "sql":
        return "sql"
    default:
        return "c"
    }
}

// colors: { keyword, string, comment, number }
function highlight(code, lang, colors) {
    var src = String(code)
    var c = colors || {}
    var kwColor = c.keyword || "#6aa8e0"
    var strColor = c.string || "#4cc38a"
    var cmtColor = c.comment || "#67727f"
    var numColor = c.number || "#e5b45a"
    var kwStr = keywordsFor(lang)
    var kwSet = {}
    if (kwStr) {
        var parts = kwStr.split(" ")
        for (var i = 0; i < parts.length; i++)
            kwSet[parts[i]] = true
    }
    var style = commentStyle(lang)
    var html = ""
    var i = 0
    var n = src.length

    function emit(cls, text) {
        var e = escapeHtml(text)
        if (!cls)
            html += e
        else {
            var col = cls === "keyword" ? kwColor
                : cls === "string" ? strColor
                : cls === "comment" ? cmtColor
                : cls === "number" ? numColor
                : ""
            html += col ? ('<span style="color:' + col + '">' + e + "</span>") : e
        }
    }

    while (i < n) {
        // Comments
        if (style === "hash" && src[i] === "#") {
            var end = src.indexOf("\n", i)
            if (end < 0) end = n
            emit("comment", src.substring(i, end))
            i = end
            continue
        }
        if (style === "sql" && src[i] === "-" && src[i + 1] === "-") {
            var endSql = src.indexOf("\n", i)
            if (endSql < 0) endSql = n
            emit("comment", src.substring(i, endSql))
            i = endSql
            continue
        }
        if (style === "c" && src[i] === "/" && src[i + 1] === "/") {
            var endC = src.indexOf("\n", i)
            if (endC < 0) endC = n
            emit("comment", src.substring(i, endC))
            i = endC
            continue
        }
        if (style === "c" && src[i] === "/" && src[i + 1] === "*") {
            var endB = src.indexOf("*/", i + 2)
            if (endB < 0) endB = n
            else endB += 2
            emit("comment", src.substring(i, endB))
            i = endB
            continue
        }

        // Strings
        if (src[i] === '"' || src[i] === "'" || src[i] === "`") {
            var q = src[i]
            var j = i + 1
            while (j < n) {
                if (src[j] === "\\") { j += 2; continue }
                if (src[j] === q) { j++; break }
                j++
            }
            emit("string", src.substring(i, j))
            i = j
            continue
        }

        // Numbers
        if ((src[i] >= "0" && src[i] <= "9") || (src[i] === "." && src[i + 1] >= "0" && src[i + 1] <= "9")) {
            var k = i
            while (k < n && /[0-9a-fA-FxX._]/.test(src[k]))
                k++
            emit("number", src.substring(i, k))
            i = k
            continue
        }

        // Identifiers / keywords
        if (/[A-Za-z_]/.test(src[i])) {
            var w = i
            while (w < n && /[A-Za-z0-9_]/.test(src[w]))
                w++
            var word = src.substring(i, w)
            emit(kwSet[word] ? "keyword" : "", word)
            i = w
            continue
        }

        // Single char / whitespace
        emit("", src[i])
        i++
    }
    return html
}
