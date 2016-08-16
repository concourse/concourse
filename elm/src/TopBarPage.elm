port module TopBarPage exposing (..)

import Navigation

import TopBar

port toggleSidebar : () -> Cmd msg
port setGroups : List String -> Cmd msg
port navigateTo : String -> Cmd msg

main : Program TopBar.Flags
main =
  Navigation.programWithFlags
    (Navigation.makeParser identity)
    { init =
        TopBar.init
          { toggleSidebar = toggleSidebar
          , setGroups = setGroups
          , navigateTo = navigateTo
          }
    , update = TopBar.update
    , urlUpdate = TopBar.urlUpdate
    , view = TopBar.view
    , subscriptions = TopBar.subscriptions
    }
