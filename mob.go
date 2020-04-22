package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const release = "0.0.2"
const configFileName = "./.mob_config.json"

type MobConfig struct {
	WipBranch        string `json: "MobWipBranch"`
	BaseBranch       string `json: "MobBaseBranch"`
	RemoteName       string `json: "MobRemoteName"`
	WipCommitMessage string `json: "MobWipCommitMessage"`
	MobNextStay      bool   `json: "MobNextStay"`
	VoiceCommand     string `json: "MobVoiceComand"`
	Debug            bool   `json: "MobDebug"`
}

//Default config values
var mobConfig = MobConfig{
	WipBranch:        "mob-session",
	BaseBranch:       getCurrentBranchName(),
	RemoteName:       "origin",
	WipCommitMessage: "mob next [ci-skip]",
	MobNextStay:      true,
	VoiceCommand:     "say",
	Debug:            false,
}

func parseEnvironmentVariables() []string {
	flagMobNextStaySet := flag.Bool("stay", false, "don't change back")
	flagMobNextSSet := flag.Bool("s", false, "(shorthand)")

	flag.Parse()

	if *flagMobNextStaySet {
		mobConfig.MobNextStay = true
	}
	if *flagMobNextSSet {
		mobConfig.MobNextStay = true
	}

	return flag.Args()
}

func main() {

	if fileExists(configFileName) {
		readConfigFile()
	}

	args := parseEnvironmentVariables()
	command := getCommand(args)
	parameter := getParameters(args)
	if mobConfig.Debug {
		fmt.Println("Args '" + strings.Join(args, " ") + "'")
		fmt.Println("command '" + command + "'")
		fmt.Println("parameter '" + strings.Join(parameter, " ") + "'")
	}

	if command == "s" || command == "start" {
		start(parameter)
		status()
	} else if command == "setup" {
		setup()
	} else if command == "n" || command == "next" {
		next()
	} else if command == "d" || command == "done" || command == "e" || command == "end" {
		done()
	} else if command == "r" || command == "reset" {
		reset()
	} else if command == "t" || command == "timer" {
		if len(parameter) > 0 {
			timer := parameter[0]
			startTimer(timer)
		}
	} else if command == "share" {
		startZoomScreenshare()
	} else if command == "h" || command == "help" || command == "--help" || command == "-h" {
		help()
	} else if command == "v" || command == "version" {
		version()
	} else {
		status()
	}
}

func startTimer(timerInMinutes string) {
	if mobConfig.Debug {
		fmt.Println("Starting timer for " + timerInMinutes + " minutes")
	}
	timeoutInMinutes, _ := strconv.Atoi(timerInMinutes)
	timeoutInSeconds := timeoutInMinutes * 60
	timerInSeconds := strconv.Itoa(timeoutInSeconds)

	command := exec.Command("sh", "-c", "( sleep "+timerInSeconds+" && "+mobConfig.VoiceCommand+" \"mob next\" && (/usr/bin/osascript -e 'display notification \"mob next\"' || /usr/bin/notify-send \"mob next\")  & )")
	if mobConfig.Debug {
		fmt.Println(command.Args)
	}
	err := command.Start()
	if err != nil {
		sayError("timer couldn't be started... (timer only works on OSX)")
		sayError(err)
	} else {
		timeOfTimeout := time.Now().Add(time.Minute * time.Duration(timeoutInMinutes)).Format("15:04")
		sayOkay(timerInMinutes + " minutes timer started (finishes at approx. " + timeOfTimeout + ")")
	}
}

func reset() {
	git("fetch", "--prune")
	git("checkout", mobConfig.BaseBranch)
	if hasMobbingBranch() {
		git("branch", "-D", mobConfig.WipBranch)
	}
	if hasMobbingBranchOrigin() {
		git("push", mobConfig.RemoteName, "--delete", mobConfig.WipBranch)
	}
}

func start(parameter []string) {
	if isSomethingToCommit() {
		sayNote("uncommitted changes")
		return
	}

	git("fetch", "--prune")
	git("pull", "--ff-only")

	if len(parameter) > 1 {
		mobConfig.WipBranch = parameter[1]
	}

	wipBranchSetup()

	if len(parameter) > 0 {
		timer := parameter[0]
		startTimer(timer)
	}

	if len(parameter) > 1 && parameter[1] == "share" {
		startZoomScreenshare()
	}

	if !fileExists(configFileName) {
		writeConfigFile()
	}
}

func setup() {
	var inputValue string

	sayInfo("Enter the wip branch name [default = " + mobConfig.WipBranch + "]:")
	readStringTrillingNewLine(&inputValue)

	if inputValue != "" {
		mobConfig.WipBranch = inputValue
		sayOkay("Wip Branch setted to " + mobConfig.WipBranch)
	} else {
		sayOkay("wip branch keeps the default value = " + mobConfig.WipBranch)
	}

	sayInfo("Enter the base branch name [default = " + mobConfig.BaseBranch + "]:")
	readStringTrillingNewLine(&inputValue)

	if inputValue != "" {
		mobConfig.BaseBranch = inputValue
		sayOkay("Base Branch setted to " + mobConfig.BaseBranch)
	} else {
		sayOkay("Base branch keeps the default value = " + mobConfig.BaseBranch)
	}

	sayInfo("Enter the remote name [default = " + mobConfig.RemoteName + "]:")
	readStringTrillingNewLine(&inputValue)

	if inputValue != "" {
		mobConfig.RemoteName = inputValue
		sayOkay("Remote name setted to " + mobConfig.RemoteName)
	} else {
		sayOkay("Remote name keeps the default value = " + mobConfig.RemoteName)
	}

	sayInfo("Enter the next staty config value [default = " + strconv.FormatBool(mobConfig.MobNextStay) + "]:")
	readStringTrillingNewLine(&inputValue)

	b, err := strconv.ParseBool(inputValue)

	if (err != nil) && (b) {
		mobConfig.MobNextStay = b
		sayOkay("Next stay setted to " + strconv.FormatBool(mobConfig.MobNextStay))
	} else {
		sayOkay("Next stay keeps the default value = " + strconv.FormatBool(mobConfig.MobNextStay))
	}

	wipBranchSetup()
	writeConfigFile()

	sayInfo("done")

}

func readStringTrillingNewLine(inputValue *string) {
	*inputValue = ""
	fmt.Scanf("%s", inputValue)
	*inputValue = strings.TrimSuffix(*inputValue, "\n")
}

func wipBranchSetup() {
	if hasMobbingBranch() && hasMobbingBranchOrigin() {
		sayInfo("rejoining mob session")
		if !isMobbing() {
			git("branch", "-D", mobConfig.WipBranch)
			git("checkout", mobConfig.WipBranch)
			git("branch", "--set-upstream-to="+mobConfig.RemoteName+"/"+mobConfig.WipBranch, mobConfig.WipBranch)
		}
	} else if !hasMobbingBranch() && !hasMobbingBranchOrigin() {
		sayInfo("create " + mobConfig.WipBranch + " from " + mobConfig.BaseBranch)
		git("checkout", mobConfig.BaseBranch)
		git("merge", mobConfig.RemoteName+"/"+mobConfig.BaseBranch, "--ff-only")
		git("branch", mobConfig.WipBranch)
		git("checkout", mobConfig.WipBranch)
		git("push", "--set-upstream", mobConfig.RemoteName, mobConfig.WipBranch)
	} else if !hasMobbingBranch() && hasMobbingBranchOrigin() {
		sayInfo("joining mob session")
		git("checkout", mobConfig.WipBranch)
		git("branch", "--set-upstream-to="+mobConfig.RemoteName+"/"+mobConfig.WipBranch, mobConfig.WipBranch)
	} else {
		sayInfo("purging local branch and start new " + mobConfig.WipBranch + " branch from " + mobConfig.BaseBranch)
		git("branch", "-D", mobConfig.WipBranch) // check if unmerged commits

		git("checkout", mobConfig.BaseBranch)
		git("merge", mobConfig.RemoteName+"/"+mobConfig.BaseBranch, "--ff-only")
		git("branch", mobConfig.WipBranch)
		git("checkout", mobConfig.WipBranch)
		git("push", "--set-upstream", mobConfig.RemoteName, mobConfig.WipBranch)
	}
}

func startZoomScreenshare() {
	commandStr := "(osascript -e 'tell application \"System Events\" to keystroke \"S\" using {shift down, command down}')"

	if runtime.GOOS == "linux" {
		commandStr = "(xdotool windowactivate $(xdotool search --name --onlyvisible 'zoom meeting') && xdotool keydown Alt s)"

	}

	command := exec.Command("sh", "-c", commandStr)

	if mobConfig.Debug {
		fmt.Println(command.Args)
	}
	err := command.Start()
	if err != nil {
		sayError("screenshare couldn't be started... (screenshare only works on OSX or Linux with xdotool installed)")
		sayError(err)
	} else {
		if runtime.GOOS == "linux" {
			sayOkay("Sharing screen with zoom (requires the global shortcut ALT+S)")
		} else {
			sayOkay("Sharing screen with zoom (requires the global shortcut SHIFT+COMMAND+S)")
		}
	}
}

func next() {
	if !isMobbing() {
		sayError("you aren't mobbing")
		return
	}

	if !isSomethingToCommit() {
		sayInfo("nothing was done, so nothing to commit")
	} else {
		git("add", "--all")
		git("commit", "--message", "\""+mobConfig.WipCommitMessage+"\"", "--no-verify")
		changes := getChangesOfLastCommit()
		git("push", mobConfig.RemoteName, mobConfig.WipBranch)
		say(changes)
	}
	showNext()

	if !mobConfig.MobNextStay {
		git("checkout", mobConfig.BaseBranch)
	}
}

func getChangesOfLastCommit() string {
	return strings.TrimSpace(silentgit("diff", "HEAD^1", "--stat"))
}

func getCachedChanges() string {
	return strings.TrimSpace(silentgit("diff", "--cached", "--stat"))
}

func done() {
	if !isMobbing() {
		sayError("you aren't mobbing")
		return
	}

	git("fetch", "--prune")

	if hasMobbingBranchOrigin() {
		if isSomethingToCommit() {
			git("add", "--all")
			git("commit", "--message", "\""+mobConfig.WipCommitMessage+"\"", "--no-verify")
		}
		git("push", mobConfig.RemoteName, mobConfig.WipBranch)

		git("checkout", mobConfig.BaseBranch)
		git("merge", mobConfig.RemoteName+"/"+mobConfig.BaseBranch, "--ff-only")
		git("merge", "--squash", "--ff", mobConfig.WipBranch)

		git("branch", "-D", mobConfig.WipBranch)
		git("push", mobConfig.RemoteName, "--delete", mobConfig.WipBranch)
		say(getCachedChanges())
		sayTodo("git commit -m 'describe the changes'")
	} else {
		git("checkout", mobConfig.BaseBranch)
		git("branch", "-D", mobConfig.WipBranch)
		sayInfo("someone else already ended your mob session")
	}

	git("rm", configFileName)
	sayInfo("mob config file removed")
}

func status() {
	if isMobbing() {
		sayInfo("mobbing in progress")

		output := silentgit("--no-pager", "log", mobConfig.BaseBranch+".."+mobConfig.WipBranch, "--pretty=format:%h %cr <%an>", "--abbrev-commit")
		say(output)
	} else {
		sayInfo("you aren't mobbing right now")
	}

	if !hasSay() {
		sayNote("text-to-speech disabled because '" + mobConfig.VoiceCommand + "' not found")
	}
}

func isSomethingToCommit() bool {
	output := silentgit("status", "--short")
	return len(strings.TrimSpace(output)) != 0
}

func isMobbing() bool {
	output := silentgit("branch")
	return strings.Contains(output, "* "+mobConfig.WipBranch)
}

func hasMobbingBranch() bool {
	output := silentgit("branch")
	return strings.Contains(output, "  "+mobConfig.WipBranch) || strings.Contains(output, "* "+mobConfig.WipBranch)
}

func hasMobbingBranchOrigin() bool {
	output := silentgit("branch", "--remotes")
	return strings.Contains(output, "  "+mobConfig.RemoteName+"/"+mobConfig.WipBranch)
}

func getGitUserName() string {
	return strings.TrimSpace(silentgit("config", "--get", "user.name"))
}

func getCurrentBranchName() string {
	return strings.TrimSuffix(silentgit("rev-parse", "--abbrev-ref", "HEAD"), "\n")
}

func showNext() {
	if mobConfig.Debug {
		say("determining next person based on previous changes")
	}
	changes := strings.TrimSpace(silentgit("--no-pager", "log", mobConfig.BaseBranch+".."+mobConfig.WipBranch, "--pretty=format:%an", "--abbrev-commit"))
	lines := strings.Split(strings.Replace(changes, "\r\n", "\n", -1), "\n")
	numberOfLines := len(lines)
	if mobConfig.Debug {
		say("there have been " + strconv.Itoa(numberOfLines) + " changes")
	}
	gitUserName := getGitUserName()
	if mobConfig.Debug {
		say("current git user.name is '" + gitUserName + "'")
	}
	if numberOfLines < 1 {
		return
	}
	var history = ""
	for i := 0; i < len(lines); i++ {
		if lines[i] == gitUserName && i > 0 {
			sayInfo("Committers after your last commit: " + history)
			sayInfo("***" + lines[i-1] + "*** is (probably) next.")
			return
		}
		if history != "" {
			history = ", " + history
		}
		history = lines[i] + history
	}
}

func help() {
	say("usage")
	say("\tmob [s]tart \t# start mobbing as typist")
	say("\tmob [-s][-stay] [n]ext \t# hand over to next typist")
	say("\tmob [d]one \t# finish mob session")
	say("\tmob [r]eset \t# resets any unfinished mob session")
	say("\tmob status \t# show status of mob session")
	say("\tmob share \t# start screenshare with zoom")
	say("\tmob help \t# prints this help")
	say("\tmob version \t# prints the version")
	say("")
	say("examples")
	say("\t mob start 10 \t# start 10 min session")
	say("\t mob start 10 share \t# start 10 min session with zoom screenshare")
	say("\t mob next \t# after 10 minutes work ...")
	say("\t mob done \t# After the work is done")

}

func version() {
	say("v" + release)
}

func silentgit(args ...string) string {
	command := exec.Command("git", args...)
	outputBinary, err := command.CombinedOutput()
	output := string(outputBinary)

	if err != nil {
		fmt.Println(output)
		fmt.Println(err)
		os.Exit(1)
	}
	return output
}

func hasSay() bool {
	command := exec.Command("which", mobConfig.VoiceCommand)
	if mobConfig.Debug {
		fmt.Println(command.Args)
	}
	outputBinary, err := command.CombinedOutput()
	output := string(outputBinary)
	if mobConfig.Debug {
		fmt.Println(output)
	}
	return err == nil
}

func git(args ...string) string {
	command := exec.Command("git", args...)
	if mobConfig.Debug {
		fmt.Println(command.Args)
	}
	outputBinary, err := command.CombinedOutput()
	output := string(outputBinary)
	if mobConfig.Debug {
		fmt.Println(output)
	}
	if err != nil {
		sayError(command.Args)
		sayError(err)
		os.Exit(1)
	} else {
		sayOkay(command.Args)
	}
	return output
}

func say(s string) {
	fmt.Println(s)
}

func sayError(s interface{}) {
	fmt.Print(" ⚡ ")
	fmt.Print(s)
	fmt.Print("\n")
}

func sayOkay(s interface{}) {
	fmt.Print(" ✓ ")
	fmt.Print(s)
	fmt.Print("\n")
}

func sayNote(s interface{}) {
	fmt.Print(" ❗ ")
	fmt.Print(s)
	fmt.Print("\n")
}

func sayTodo(s interface{}) {
	fmt.Print(" ☐ ")
	fmt.Print(s)
	fmt.Print("\n")
}

func sayInfo(s string) {
	fmt.Print(" > ")
	fmt.Print(s)
	fmt.Print("\n")
}

func getCommand(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return args[0]
}

func getParameters(args []string) []string {
	if len(args) == 0 {
		return args
	}
	return args[1:]
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func readConfigFile() {
	configFile, _ := ioutil.ReadFile(configFileName)
	_ = json.Unmarshal([]byte(configFile), &mobConfig)

	sayInfo(mobConfig.BaseBranch)
}

func writeConfigFile() {
	sayInfo("creating default mob config file...")
	configFile, _ := json.MarshalIndent(mobConfig, "", " ")
	_ = ioutil.WriteFile(configFileName, configFile, 0644)
	git("add", "--all")
	git("commit", "--message", "mob started", "--no-verify")

	sayOkay("default mob config file created")
}
