# Web-build
[![Go Report Card](https://goreportcard.com/badge/github.com/ryanmitchener/web-build)](https://goreportcard.com/report/github.com/ryanmitchener/web-build)

Web-build is a simple task runner/build system written in Go, built for web projects to allow for build targets and managing of specific assets.


## Updates
### 1.2
- Added "input" option to js-minify

### 1.1
- Added "shell" action to run command line commands inside of tasks (i.e. TypeScript, rollup, .etc)
- Added more errors for configurations
- Targets are no longer required to use web-build
- Fixed `web-build init`


## Goals
- Allow web projects to have build targets to allow for different versions of an app or rebranding/reselling
- Manage web assets with a few predefined tasks
  - Minification of JavaScript
  - Concatenation of assets (specifically JavaScript)
  - Generation of SASS
  - Collation of any type of file (images/fonts/.etc)
  - Run command line commands
- Watch for file changes and re-run tasks automatically


## Disclaimers
- As this is a personal project built to suit my needs at the current moment, I do not expect to add new features quickly or consistently.
- This is my first project using Go. This code does not necessarily represent idiomatic or even good Go code. I did however thoroughly enjoy working with the language.


## Setup
In order to use Web-build from this repository, you will need to install [Go](https://golang.org/dl/).
Once Go is installed, clone this git repository, change the directory to the repository directory
and run the following command in the terminal:

```shell
go build ./
```

This command will generate the binary web-build and place it in your current working directory. You may
now move this binary wherever you want (preferably to a location in your PATH).


## Usage
For help in using the binary, run the following command:

```shell
web-build -h
```

To get started with a new project, create a new folder, change your current directory to it and run

```shell
web-build init
```

This will initialize an empty project with a default `web-build.json` file. This is the file where you will
place all of the configuration for your project.


### web-build.json
Every `web-build.json` is comprised of a few required top-level elements:
- `templateVersion` The version of the template currently being used. This is specifically for backwards compatibility and serves no use at the moment.
- `srcDir` The directory for all of the source files in the project
- `buildDir` The directory where all source files will be compiled to. This is the directory you will serve your web project from.
- `target` The target to build.
- `targets` The list of targets with their dependencies
- `tasks` The list of tasks to run


#### Assumptions
- All target directories live directly inside of `[srcDir]`
- Globs are relative paths. For applications without targets, globs are relative to the `srcDir`. For applications with targets, globs are relative to a target directory. For example, if you had a target "*test*" and a glob "*/innerFolder*", the glob would look in "*[srcDir]/test/innerFolder*".
- Every task is completely isolated from the others and can (and will) run concurrently
- Path separators in `web-build.json` are UNIX separators "/"


#### Build Targets
Adding targets allows a user to customize images, templates, CSS, and even JavaScript allowing for a
different version of the application. During the build process, targets will trace their dependency 
tree back to the top-most level and then walk down the tree adding and replacing files when 
necessary. **_note: no files are removed from a target, only added or replaced._** The resulting
file set is a merging of the current target's files and its dependencies'. The generated file set
is then processed according to its file types and output to the `[buildDir]` directory. **Files in the** 
`[buildDir]` **directory should not be modified.**


#### Adding a New Target
To add a target, modify the `targets` object in `web-build.json` and add a new target. Targets must contain a
`dependency` property (the target your new target will be based on).

Once the target is defined, add a folder to the `[srcDir]` directory matching
the name given to your new build target. Any parent target file you wish to replace must match 
the same path as the parent target's file. For example, if you wanted to replace the following
image: `./[srcDir]/main/images/my-cat.jpg`, you would need to create the image with the following
path: `./[srcDir]/your-new-target/images/my-cat.jpg`.


#### Changing the Current Build Target
To change the current target, modify the `target` property in `web-build.json` to reflect the
target you wish to build. Then you may run `web-build` to compile the application.


#### Tasks
A task can have any name. This name will be printed in the console during execution.
Tasks will run concurrently and should be considered completely isolated. For this reason, you should not
have two tasks that manipulate the same files.

Tasks are made up of two properties: `globs` and `actions`. The `globs` property is an array of strings
while the `actions` property is an array of action objects.


#### Globs
Globs are a way of selecting files based on some simplified path selectors. In `web-build` globs are
sugar-coated regular expressions. `web-build` manipulates globs in the following ways:

- `.` is replaced with `\.`
- `*` is replaced with `[^\/]*` allowing for selecting any file in a specific directory.
- `**` is replaced with `.*` allowing for selecting any file recursively through directories.
- All globs are automatically suffixed with the `$` character denoting the end of the match.
- Any glob with a `!` character as the first character will exclude any matches for the current result-set.

For example, the following array of globs will select all JavaScript files except for already minified JavaScript files.
`["/assets/js/**.js", "!/assets/js/*.min.js"]`


#### Actions
Actions are run on glob results sequentially. The output files of an action are passed as the input to the next action. Action objects have the following properties:
`action` The name of the action to run
`options` This parameter is only required if the action requires parameters to be passed.

There are only a few actions defined at the moment:
- `collate` Collects all files from the dependency and places them in their respective folder in the `[buildDir]`. This is the most basic of actions and essentially just places the files into the `[buildDir]` directory. `collate` takes the optional parameter `output`. This is the desired base output directory for all of the collated files.
- `concat` Concatenates all files. `concat` takes an optional parameter of `separator` and a required parameter of `output`. `separator` defines the separator as a string to use in between files. `output` specifies the directory and file name to create relative to the `[buildDir]`.
- `js-minify` Minify JavaScript files. `js-minify` takes the optional parameter of `input` and `output`. If `input` is specified, js-minify will ignore the passed in files from the previous action and instead use the provided input file string. If `output` is specified, it will only be used if there is only one file going into it (for example: when the previous action is a `concat` action). If `output` is omitted, the files will simply append .min.js to the filename.
- `sass` Compile SASS files. `sass` takes no parameters. `sass` first collates glob files before compiling them with libsass. This allows you to have a different `variables.scss` per build target that can be included in another SASS sheet using a simple relative path.
- `shell` Run a shell command. `shell` takes one parameter of `command`. There are two placeholders that may be used in your commands: `{FILE}` and `{FILES}`. A command using the `{FILE}` placeholder will be run against all matching files. This may be a slow process and is not the preferable option. A command using the `{FILES}` placeholder will run a command against a white-space separated list of all matching files. For example: `tsc -outDir ./build/ts {FILES}` will be replaced with `tsc --outDir ./build/ts ./src/file1 ./src/file2 ./src/file3`. <br><br>At the moment the `shell` action does not support returning a list of affected files as most of the other actions do. Instead the input files are passed to the next action unchanged.<br><br>In addition, `shell` actions that require different commands per platform are not supported at this time. 


## License
Web-build released under the MIT license.