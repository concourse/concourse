module Layout exposing (Model, Msg, init, update, urlUpdate, view, subscriptions)

import Html exposing (Html)
import Html.App
import TopBar
import SideBar
import Routes
import SubPage

type alias Model =
  { sub : SubPage.Page
  , subModel : SubPage.Model
  -- , topModel : TopBar.Model
  -- , sideModel : SideBar.Model
  }

type Msg
  = SubMsg SubPage.Msg
  | TopMsg TopBar.Msg
  | SideMsg SideBar.Msg

init : SubPage.Page -> Routes.Route -> (Model, Cmd (Msg))
init sub route =
  let
    (subModel, subCmd) =
      sub.init route
      -- case route of
      --   LoginR ->
      --     sub.init Login.TeamSelectionPage
      --
      --   TeamLogin teamName ->
      --     Debug.log "in team login" <|
      --       sub.init (Login.LoginPage teamName)

    -- (topModel, topCmd) =
    --   TopBar.init route
    --
    -- (sideModel, sideCmd) =
    --   SideBar.init
  in
    ( { sub = sub
      , subModel = subModel
      -- , topModel = topModel
      -- , sideModel = sideModel
      }
    , Cmd.batch
        [ Cmd.map SubMsg subCmd
--         , Cmd.map TopMsg topCmd
--         , Cmd.map SideMsg sideCmd
        ]
    )

update : Msg -> Model -> (Model, Cmd (Msg))
update msg model =
  case msg of
    SubMsg m ->
      Debug.log("got sub message") <|
        let
          (subModel, subCmd) = model.sub.update m model.subModel
        in
          ({ model | subModel = subModel }, Cmd.map SubMsg subCmd)

    TopMsg m ->
      (model, Cmd.none)

    SideMsg m ->
      (model, Cmd.none)

urlUpdate : Routes.Route -> Model -> (Model, Cmd (Msg))
urlUpdate route model =
  let
    (subModel, subCmd) =
      model.sub.init route
      -- case route of
      --   LoginR ->
      --     model.sub.init
      --
      --   TeamLogin teamName ->
      --     Debug.log "in team login" <|
      --       model.sub.init (Login.LoginPage teamName)

    -- (topModel, topCmd) =
    --   TopBar.init route
    --
    -- (sideModel, sideCmd) =
    --   SideBar.init
  in
    ( { model | subModel = subModel }
    , Cmd.batch
        [ Cmd.map SubMsg subCmd
        ]
    )

view : Model -> Html (Msg)
view model =
  Html.App.map SubMsg (model.sub.view model.subModel)

subscriptions : Model -> Sub (Msg)
subscriptions model =
  Sub.none
