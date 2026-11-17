package yaml_ast

import "testing"

func TestFormat_TopLevelSpacing(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "adjacent_top_level_keys_get_one_blank_line",
			in: `nixpkgs:
  default: abc
packages:
  - go
env:
  FOO: bar
`,
			want: `nixpkgs:
  default: abc

packages:
  - go

env:
  FOO: bar
`,
		},
		{
			name: "already_formatted_input_is_unchanged",
			in: `nixpkgs:
  default: abc

packages:
  - go
`,
			want: `nixpkgs:
  default: abc

packages:
  - go
`,
		},
		{
			name: "nested_keys_do_not_split",
			in: `docker-mode:
  ports:
    - 8080:8080
  platform: linux/amd64
`,
			want: `docker-mode:
  ports:
    - 8080:8080
  platform: linux/amd64
`,
		},
		{
			name: "blank_lines_inside_block_scalar_are_kept",
			in: `builds:
  app:
    command: |
      echo one

      echo two
    paths:
      - ./bin/app
mounts:
  - source: /a
`,
			want: `builds:
  app:
    command: |
      echo one

      echo two
    paths:
      - ./bin/app

mounts:
  - source: /a
`,
		},
		{
			name: "head_comment_stays_with_its_key",
			in: `# shared settings
env:
  FOO: bar
# builds
builds:
  app:
`,
			want: `# shared settings
env:
  FOO: bar

# builds
builds:
  app:
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(Format([]byte(c.in))); got != c.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}
