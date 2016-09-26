module Layout exposing (Model, Msg, init, update, urlUpdate, view, subscriptions)

import Html exposing (Html)
import Html.Attributes as Attributes exposing (class, id)
import Html.App
import Login exposing (Msg(..))
import TopBar
import SideBar
import Routes
import SubPage

type alias Model =
  { subModel : SubPage.Model
  , topModel : TopBar.Model
  , sideModel : SideBar.Model
  , sidebarVisible : Bool
  }

type Msg
  = SubMsg SubPage.Msg
  | TopMsg TopBar.Msg
  | SideMsg SideBar.Msg

init : Routes.ConcourseRoute -> (Model, Cmd (Msg))
init route =
  let
    (subModel, subCmd) =
      SubPage.init route

    (topModel, topCmd) =
      TopBar.init route

    (sideModel, sideCmd) =
      SideBar.init
  in
    ( { subModel = subModel
      , topModel = topModel
      , sideModel = sideModel
      , sidebarVisible = False
      }
    , Cmd.batch
        [ Cmd.map SubMsg subCmd
        , Cmd.map TopMsg topCmd
        , Cmd.map SideMsg sideCmd
        ]
    )

update : Msg -> Model -> (Model, Cmd (Msg))
update msg model =
  case msg of
    -- handle cross-component interactions
    TopMsg TopBar.ToggleSidebar ->
      ( { model
        | sidebarVisible = not model.sidebarVisible
        }
      , Cmd.none
      )
    SubMsg (SubPage.LoginMsg (Login.LoginTokenReceived (Ok val))) ->
      let
        (subModel, subCmd) =
          SubPage.update (SubPage.LoginMsg (Login.LoginTokenReceived (Ok val))) model.subModel
      in
        ( { model
          | subModel = subModel
          }
        , Cmd.batch
            [ Cmd.map TopMsg TopBar.fetchUser
            , Cmd.map SideMsg SideBar.fetchPipelines
            , Cmd.map SubMsg subCmd
            ]
        )
    -- otherwise, pass down
    SubMsg m ->
      Debug.log("got sub message") <|
        let
          (subModel, subCmd) = SubPage.update (Debug.log "message: " m) model.subModel
        in
          ({ model | subModel = subModel }, Cmd.map SubMsg subCmd)

    TopMsg m ->
      let
        (topModel, topCmd) = TopBar.update m model.topModel
      in
        ({ model | topModel = topModel }, Cmd.map TopMsg topCmd)

    SideMsg m ->
      let
        (sideModel, sideCmd) = SideBar.update m model.sideModel
      in
        ({ model | sideModel = sideModel }, Cmd.map SideMsg sideCmd)

urlUpdate : Routes.ConcourseRoute -> Model -> (Model, Cmd (Msg))
urlUpdate route model =
  let
    (newSubmodel, cmd) =
      if routeMatchesModel route model then
        SubPage.urlUpdate route model.subModel
      else
        SubPage.init route
    (newTopModel, tCmd) =
      TopBar.urlUpdate route model.topModel
  in
    ( { model
      | subModel = newSubmodel
      , topModel = newTopModel
      }
    , Cmd.batch
        [ Cmd.map SubMsg cmd
        , Cmd.map TopMsg tCmd
        ]
    )

view : Model -> Html (Msg)
view model =
  let sidebarVisibileAppendage =
    case model.sidebarVisible of
      True ->
        " visible"
      False ->
        ""
  in
    Html.div [ class "content-frame" ]
      [ Html.div [ id "top-bar-app" ]
          [ Html.App.map TopMsg (TopBar.view model.topModel) ]
      , Html.div [ class "bottom" ]
          [ Html.div
              [ id "pipelines-nav-app"
              , class <| "sidebar test" ++ sidebarVisibileAppendage
              ]
              [ Html.App.map SideMsg (SideBar.view model.sideModel) ]
          , Html.div [ id "content" ]
              [ Html.div [ id "subpage" ]
                  [ Html.App.map SubMsg (SubPage.view model.subModel) ]
              ]
          ]
      ]

subscriptions : Model -> Sub (Msg)
subscriptions model =
  Sub.none -- TODO this maybe blank? idk


routeMatchesModel : Routes.ConcourseRoute -> Model -> Bool
routeMatchesModel route model =
  case (route.logical, model.subModel) of
    (Routes.SelectTeam, SubPage.SelectTeamModel _) ->
      True
    (Routes.TeamLogin _, SubPage.LoginModel _) ->
      True
    (Routes.Pipeline _ _, SubPage.PipelineModel _) ->
      True
    _ -> False
