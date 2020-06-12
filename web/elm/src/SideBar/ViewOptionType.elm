module SideBar.ViewOptionType exposing (ViewOption(..), viewOptions)

-- This only exists to avoid an import cycle


type ViewOption
    = ViewNonArchivedPipelines
    | ViewArchivedPipelines


viewOptions : List ViewOption
viewOptions =
    [ ViewNonArchivedPipelines
    , ViewArchivedPipelines
    ]
