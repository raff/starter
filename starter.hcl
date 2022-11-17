debug = true

environment = {
    what = "hello"
    who = "world"
    env = "$${FROM:-default}"
}

application "example-1" {
    color = "green"
    program = "./example.bash"
    args = ["$${what}", "$${who}"]
    stdout-idle = 5
    stderr-idle = 8
    min-wait = 10
}

application "example-2" {
    color = "cyan"
    program = "ls"
    args = ["-l", "/etc/"]
    stdout-idle = 5
    stderr-idle = 8
    min-wait = 7
}

application "test" {
    color = "red"
    program = "./example.bash"
    args = ["not now", "$${env}"]
    manual = true
}
