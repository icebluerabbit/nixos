_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.picker = {
    aspect-name = "cli.neovim.plugins.snacks.picker";
    homeManager =
      _:
      let
        key = key: action: desc: {
          inherit key;
          action = "<cmd>${action}<cr>";
          options = { inherit desc; };
        };
      in
      {
        programs.nixvim.plugins.snacks.settings.picker = {
          enabled = true;
          ui_select = true;
        };

        programs.nixvim.keymaps = [
          # Top Pickers
          (key "<leader><space>" "lua Snacks.picker.smart()" "Smart Find Files")
          (key "<leader>," "lua Snacks.picker.buffers()" "Buffers")
          (key "<leader>/" "lua Snacks.picker.grep()" "Grep")
          (key "<leader>:" "lua Snacks.picker.command_history()" "Command History")
          (key "<leader>n" "lua Snacks.notifier.show_history()" "Notification History")
          (key "<leader>e" "lua Snacks.explorer()" "File Explorer")
          # Find
          (key "<leader>fb" "lua Snacks.picker.buffers()" "Buffers")
          (key "<leader>fc" "lua Snacks.picker.files({ cwd = vim.fn.stdpath(\"config\") })"
            "Find Config File"
          )
          (key "<leader>ff" "lua Snacks.picker.files()" "Find Files")
          (key "<leader>fg" "lua Snacks.picker.git_files()" "Find Git Files")
          (key "<leader>fp" "lua Snacks.picker.projects()" "Projects")
          (key "<leader>fr" "lua Snacks.picker.recent()" "Recent")
          # Git
          (key "<leader>gb" "lua Snacks.picker.git_branches()" "Git Branches")
          (key "<leader>gl" "lua Snacks.picker.git_log()" "Git Log")
          (key "<leader>gL" "lua Snacks.picker.git_log_line()" "Git Log Line")
          (key "<leader>gs" "lua Snacks.picker.git_status()" "Git Status")
          (key "<leader>gS" "lua Snacks.picker.git_stash()" "Git Stash")
          (key "<leader>gd" "lua Snacks.picker.git_diff()" "Git Diff (Hunks)")
          (key "<leader>gf" "lua Snacks.picker.git_log_file()" "Git Log File")
          # GitHub
          (key "<leader>gi" "lua Snacks.picker.gh_issue()" "GitHub Issues (open)")
          (key "<leader>gI" "lua Snacks.picker.gh_issue({ state = \"all\" })" "GitHub Issues (all)")
          (key "<leader>gp" "lua Snacks.picker.gh_pr()" "GitHub Pull Requests (open)")
          (key "<leader>gP" "lua Snacks.picker.gh_pr({ state = \"all\" })" "GitHub Pull Requests (all)")
          # Grep
          (key "<leader>sb" "lua Snacks.picker.lines()" "Buffer Lines")
          (key "<leader>sB" "lua Snacks.picker.grep_buffers()" "Grep Open Buffers")
          (key "<leader>sg" "lua Snacks.picker.grep()" "Grep")
          (
            (key "<leader>sw" "lua Snacks.picker.grep_word()" "Visual selection or word")
            // {
              mode = [
                "n"
                "x"
              ];
            }
          )
          # Search
          (key "<leader>s\"" "lua Snacks.picker.registers()" "Registers")
          (key "<leader>s/" "lua Snacks.picker.search_history()" "Search History")
          (key "<leader>sa" "lua Snacks.picker.autocmds()" "Autocmds")
          (key "<leader>sc" "lua Snacks.picker.command_history()" "Command History")
          (key "<leader>sC" "lua Snacks.picker.commands()" "Commands")
          (key "<leader>sd" "lua Snacks.picker.diagnostics()" "Diagnostics")
          (key "<leader>sD" "lua Snacks.picker.diagnostics_buffer()" "Buffer Diagnostics")
          (key "<leader>sh" "lua Snacks.picker.help()" "Help Pages")
          (key "<leader>sH" "lua Snacks.picker.highlights()" "Highlights")
          (key "<leader>si" "lua Snacks.picker.icons()" "Icons")
          (key "<leader>sj" "lua Snacks.picker.jumps()" "Jumps")
          (key "<leader>sk" "lua Snacks.picker.keymaps()" "Keymaps")
          #(key "<leader>sl" "lua Snacks.picker.loclist()" "Location List")
          (key "<leader>sm" "lua Snacks.picker.marks()" "Marks")
          (key "<leader>sM" "lua Snacks.picker.man()" "Man Pages")
          (key "<leader>sq" "lua Snacks.picker.qflist()" "Quickfix List")
          (key "<leader>sR" "lua Snacks.picker.resume()" "Resume")
          (key "<leader>su" "lua Snacks.picker.undo()" "Undo History")
          (key "<leader>uC" "lua Snacks.picker.colorschemes()" "Colorschemes")

        ];
      };
  };
}
