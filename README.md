# reee-evolution
Email filter for Evolution

JavaScript: https://github.com/dop251/goja

Rough design of JavaScript part without any kind of import/require or
code sharing.

    --- init ---
    1. For each discovered source file ~/.config/reee/rules/foo.js,
       create an instance of Runtime.
    2. Export rule registration hook into Runtime using Runtime.Set.
    3. Compile and run that file (foo.js) into that Runtime, which
       should result in the JavaScript code calling the registration
       hook registered in that Runtime, and thus registering one or
       more rule functions for that Runtime.
    4. This puts us in a state like:
            foo.js -> [Runtime#1] -> [group1.rule1, group1.rule2, group2.rule1]
            bar.js -> [Runtime#2] -> [group3.rule1]
       which is equivalent to:
            [group1.rule1] -> [Runtime#1]
            [group1.rule2] -> [Runtime#1]
            [group2.rule1] -> [Runtime#1]
            [group3.rule2] -> [Runtime#2]

    --- concurrency ---
    Individual Runtimes are not Goroutine safe, which means it is not
    safe to evaluate more than one rule from the same Runtime at the
    same time. However, by loading each JS file into its own Runtime,
    it gives us some flexibility to run things concurrently, since at
    least rules in separate files (separate Runtimes) can be run at the
    same time.