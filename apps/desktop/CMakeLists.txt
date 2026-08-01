cmake_minimum_required(VERSION 3.24)
project(openinfer-studio VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Quick QuickControls2 WebSockets Widgets)

qt_standard_project_setup(REQUIRES 6.5)

qt_add_executable(openinfer-studio
    main.cpp
    resources.qrc
)

target_link_libraries(openinfer-studio PRIVATE
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::WebSockets
    Qt6::Widgets
)

set_target_properties(openinfer-studio PROPERTIES
    WIN32_EXECUTABLE TRUE
    MACOSX_BUNDLE TRUE
    MACOSX_BUNDLE_BUNDLE_NAME "OpenInfer Studio"
    MACOSX_BUNDLE_GUI_IDENTIFIER "org.openinfer.studio"
)

install(TARGETS openinfer-studio
    BUNDLE DESTINATION .
    RUNTIME DESTINATION bin
)
