package algorithm

// NOTE: we're effectively ignoring check_order here and relying on
// build history - be careful when doing #413 that we don't go
// 'back in time'
//
// QUESTION: is version_md5 worth it? might it be surprising
// that all it takes is (resource name, version identifier)
// to consider something 'passed'?

type version struct {
	ID int

	VouchedForBy map[int]bool
}

func newVersion(id int) *version {
	return &version{
		ID: id,

		VouchedForBy: map[int]bool{},
	}
}

func Resolve(db *VersionsDB, inputConfigs InputConfigs) ([]*version, bool, error) {
	versions := make([]*version, len(inputConfigs))

	resolved, err := resolve(0, db, inputConfigs, versions)
	if err != nil {
		return nil, false, err
	}

	if resolved {
		return versions, true, nil
	}

	return nil, false, nil
}

func resolve(depth int, db *VersionsDB, inputConfigs InputConfigs, candidates []*version) (bool, error) {
	// NOTE: this is probably made most efficient by doing it in order of inputs
	// with jobs that have the broadest output sets, so that we can pin the most
	// at once
	//
	// NOTE 2: probably also makes sense to go over jobs with the fewest builds
	// first, so that we give up more quickly
	//
	// NOTE 3: maybe also select distinct build outputs so we don't waste time on
	// the same thing (i.e. constantly re-triggered build)

	for i, inputConfig := range inputConfigs {
		debug := func(messages ...interface{}) {
			// log.Println(
			// 	append(
			// 		[]interface{}{
			// 			strings.Repeat("-", depth) + fmt.Sprintf("[%s]", inputConfig.Name),
			// 		},
			// 		messages...,
			// 	)...,
			// )
		}

		if len(inputConfig.Passed) == 0 {
			// coming from recursive call; already set to the latest version
			if candidates[i] != nil {
				continue
			}

			var versionID int
			if inputConfig.PinnedVersionID != 0 {
				// pinned
				// TODO: do we need to verify that the id exists?
				versionID = inputConfig.PinnedVersionID
			} else {
				// there are no passed constraints, so just take the latest version
				var err error
				versionID, err = db.LatestVersionOfResource(inputConfig.ResourceID)
				if err != nil {
					return false, nil
				}
			}

			debug("setting candidate", i, "to unconstrained version", versionID)
			candidates[i] = newVersion(versionID)
			continue
		}

		for jobID := range inputConfig.Passed {
			if candidates[i] != nil {
				// coming from recursive call; we've already got a candidate
				if candidates[i].VouchedForBy[jobID] {
					debug("job", jobID, i, "already vouched for", candidates[i].ID)
					// we've already been here; continue to the next job
					continue
				} else {
					debug("job", jobID, i, "has not vouched for", candidates[i].ID)
				}
			} else {
				debug(i, "has no candidate yet")
			}

			// loop over previous output sets, latest first
			builds, err := db.SuccessfulBuilds(jobID)
			if err != nil {
				return false, err
			}

			for _, buildID := range builds {
				outputs, err := db.BuildOutputs(buildID)
				if err != nil {
					return false, err
				}

				debug("job", jobID, "trying build", jobID, buildID)

				restore := map[int]*version{}

				var mismatch bool

				// loop over the resource versions that came out of this build set
			outputs:
				for _, output := range outputs {
					debug("build", buildID, "output", output.ResourceID, output.VersionID)

					// try to pin each candidate to the versions from this build
					for c, candidate := range candidates {
						if inputConfigs[c].ResourceID != output.ResourceID {
							// unrelated to this output
							continue
						}

						if !inputConfigs[c].Passed.Contains(jobID) {
							// this candidate is unaffected by the current job
							debug("independent", inputConfigs[c].Passed.String(), jobID)
							continue
						}

						if candidate != nil && candidate.ID != output.VersionID {
							// don't return here! just try the next output set. it's possible
							// we just need to use an older output set.
							debug("mismatch")
							mismatch = true
							break outputs
						}

						// if this doesn't work out, restore it to either nil or the
						// candidate *without* the job vouching for it
						restore[c] = candidate

						// make a copy
						//
						// TODO: this might not actually be necessary; we should be able to
						// leave the job vouched regardless, because it *has* to at the end
						// anyway - the mark is to just prevent it from trying the job
						// again, which is what we want
						debug("setting candidate", c, "to", output.VersionID)
						candidates[c] = newVersion(output.VersionID)

						// carry over the vouchers
						if candidate != nil {
							for vJobID := range candidate.VouchedForBy {
								candidates[c].VouchedForBy[vJobID] = true
							}
						}

						// vouch for it ourselves
						debug("job", jobID, "vouching for", output.ResourceID, "version", output.VersionID)
						candidates[c].VouchedForBy[jobID] = true
					}
				}

				// we found a candidate for ourselves and the rest are OK too - recurse
				if candidates[i] != nil && !mismatch {
					debug("recursing")

					resolved, err := resolve(depth+1, db, inputConfigs, candidates)
					if err != nil {
						return false, err
					}

					if resolved {
						// we've got a match for the rest of the inputs!
						return true, nil
					}
				}

				debug("restoring")

				for c, version := range restore {
					// either there was a mismatch or resolving didn't work; go on to the
					// next output set
					debug("restoring candidate", c, "to", version)
					candidates[c] = version
				}
			}

			// we've exhausted all the builds and never found a matching input set;
			// time to give up
			return false, nil
		}
	}

	// go to the end of all the inputs - all is well!
	return true, nil
}
