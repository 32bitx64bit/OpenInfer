// OpenInfer Studio — minimal Qt bootstrap.
// Launches openinfer-core, passes token/port into QML, kills the backend on exit.
// Application logic lives in the Go backend; keep this file free of it.

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QGuiApplication>
#include <QJsonDocument>
#include <QJsonObject>
#include <QMessageBox>
#include <QProcess>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QRandomGenerator>
#include <QTcpServer>
#include <QTimer>

namespace {

QString generateToken()
{
    QByteArray raw(16, 0);
    QRandomGenerator::system()->fillRange(reinterpret_cast<quint32*>(raw.data()), 4);
    return raw.toHex();
}

quint16 pickFreePort()
{
    QTcpServer probe;
    if (!probe.listen(QHostAddress::LocalHost, 0))
        return 0;
    quint16 port = probe.serverPort();
    probe.close();
    return port;
}

QString locateBackend(const QString &appDir)
{
#ifdef Q_OS_WIN
    const QString name = QStringLiteral("openinfer-core.exe");
#else
    const QString name = QStringLiteral("openinfer-core");
#endif
    const QString env = qEnvironmentVariable("OPENINFER_CORE");
    if (!env.isEmpty() && QFileInfo::exists(env))
        return env;
    const QStringList candidates = {
        appDir + QLatin1Char('/') + name,
        appDir + QStringLiteral("/../core/") + name,
        appDir + QStringLiteral("/core/") + name,
    };
    for (const QString &c : candidates)
        if (QFileInfo::exists(c))
            return QDir(c).canonicalPath();
    return {};
}

int fatal(const QString &title, const QString &message)
{
    QMessageBox box(QMessageBox::Critical, title, message, QMessageBox::Ok);
    box.exec();
    return 1;
}

} // namespace

int main(int argc, char *argv[])
{
    // High-DPI and application identity.
    QGuiApplication::setApplicationName(QStringLiteral("openinfer-studio"));
    QGuiApplication::setApplicationDisplayName(QStringLiteral("OpenInfer Studio"));
    QGuiApplication::setOrganizationName(QStringLiteral("OpenInfer"));
    QGuiApplication::setApplicationVersion(QStringLiteral("0.1.0"));

    QGuiApplication app(argc, argv);

    const QString token = generateToken();
    const quint16 port = pickFreePort();
    if (port == 0)
        return fatal(QStringLiteral("OpenInfer Studio"),
                     QStringLiteral("Could not allocate a local port for the backend."));

    const QString backendPath = locateBackend(QCoreApplication::applicationDirPath());
    if (backendPath.isEmpty())
        return fatal(QStringLiteral("OpenInfer Studio"),
                     QStringLiteral("The backend executable (openinfer-core) was not found.\n\n"
                                    "Reinstall the application or set OPENINFER_CORE for development."));

    QProcess backend;
    backend.setProgram(backendPath);
    backend.setArguments({
        QStringLiteral("--token"), token,
        QStringLiteral("--port"), QString::number(port),
        QStringLiteral("--ppid"), QString::number(QCoreApplication::applicationPid()),
    });
    // Merge stderr into stdout so diagnostics arrive in one stream.
    backend.setProcessChannelMode(QProcess::MergedChannels);
    backend.start();

    // Backend prints {"ready":true,...}; keep extra output for fatal dialogs.
    QByteArray backendOutput;
    QObject::connect(&backend, &QProcess::readyRead, &app, [&] {
        backendOutput += backend.readAll();
    });

    if (!backend.waitForStarted(10000))
        return fatal(QStringLiteral("OpenInfer Studio"),
                     QStringLiteral("Failed to start the backend:\n%1").arg(backend.errorString()));

    // Up to 30s: first-run migrations run before the readiness line.
    bool ready = false;
    {
        QEventLoop loop;
        QTimer timeout;
        timeout.setSingleShot(true);
        QObject::connect(&timeout, &QTimer::timeout, &loop, &QEventLoop::quit);
        QObject::connect(&backend, &QProcess::readyRead, &loop, &QEventLoop::quit);
        QObject::connect(&backend, &QProcess::finished, &loop, &QEventLoop::quit);
        timeout.start(30000);
        while (!ready && timeout.isActive() && backend.state() == QProcess::Running) {
            loop.exec();
            if (backendOutput.contains("\"ready\":true")) {
                ready = true;
                break;
            }
            if (backendOutput.contains("\"ready\":false"))
                break;
        }
    }
    if (!ready) {
        QString detail = QString::fromUtf8(backendOutput).trimmed();
        if (detail.size() > 2000)
            detail = detail.right(2000);
        backend.kill();
        backend.waitForFinished(3000);
        return fatal(QStringLiteral("OpenInfer Studio"),
                     QStringLiteral("The backend did not become ready.\n\nBackend output:\n%1")
                         .arg(detail.isEmpty() ? QStringLiteral("(no output)") : detail));
    }

    // Kill backend with the UI; parent-death watchdog is the backup.
    QObject::connect(&app, &QCoreApplication::aboutToQuit, &app, [&] {
        backend.terminate();
        if (!backend.waitForFinished(4000))
            backend.kill();
    });

    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty(QStringLiteral("apiBase"),
                                             QStringLiteral("http://127.0.0.1:%1").arg(port));
    engine.rootContext()->setContextProperty(QStringLiteral("wsBase"),
                                             QStringLiteral("ws://127.0.0.1:%1").arg(port));
    engine.rootContext()->setContextProperty(QStringLiteral("apiToken"), token);

    QObject::connect(&engine, &QQmlApplicationEngine::objectCreationFailed, &app,
                     [] { QCoreApplication::exit(2); }, Qt::QueuedConnection);
    engine.load(QUrl(QStringLiteral("qrc:/qml/Main.qml")));
    return app.exec();
}
